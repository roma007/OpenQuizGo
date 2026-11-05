package familyadmin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/admin"
)

type Summary struct {
	FamilyID         string     `json:"family_id"`
	OwnerID          string     `json:"owner_id"`
	MemberStatus     string     `json:"member_status"`
	MembersCount     int        `json:"members_count"`
	ChildrenCount    int        `json:"children_count"`
	DevicesCount     int        `json:"devices_count"`
	MembershipStatus string     `json:"membership_status,omitempty"`
	PlanName         string     `json:"plan_name,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type Member struct {
	AccountID string    `json:"account_id"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

type Child struct {
	ID        string    `json:"id"`
	Nickname  string    `json:"nickname"`
	Stage     string    `json:"stage"`
	GradeTag  string    `json:"grade_tag,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Device struct {
	DeviceID       string     `json:"device_id"`
	Platform       string     `json:"platform"`
	Status         string     `json:"status"`
	ChildID        string     `json:"child_id"`
	ChildName      string     `json:"child_name"`
	BoundAt        time.Time  `json:"bound_at"`
	TokenExpiresAt *time.Time `json:"token_expires_at,omitempty"`
}

type ListResult struct {
	Items []Summary `json:"items"`
	Total int       `json:"total"`
}

type Detail struct {
	Summary
	MemberList []Member `json:"members"`
	ChildList  []Child  `json:"children"`
	DeviceList []Device `json:"devices"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) Routes(r chi.Router, guard *admin.Service) {
	r.Group(func(r chi.Router) {
		r.Use(guard.Auth)
		r.With(guard.Require("family.read")).Get("/admin/families", s.handleList)
		r.With(guard.Require("family.read")).Get("/admin/families/{id}", s.handleGet)
	})
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	result, err := s.List(r.Context(), r.URL.Query().Get("query"), limit, offset)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	detail, err := s.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Service) List(ctx context.Context, query string, limit, offset int) (ListResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	filter := `where ($1 = ''
		or f.id::text ilike '%' || $1 || '%'
		or f.owner_id::text ilike '%' || $1 || '%'
		or exists (select 1 from child_profile cp where cp.family_id = f.id and cp.nickname ilike '%' || $1 || '%')
		or exists (
		    select 1 from device_bind db
		    join child_profile cp on cp.id = db.child_id
		    where cp.family_id = f.id and db.device_id ilike '%' || $1 || '%'))`
	var total int
	if err := s.pool.QueryRow(ctx, `select count(*)::int from family f `+filter, query).Scan(&total); err != nil {
		return ListResult{}, err
	}
	rows, err := s.pool.Query(ctx, `
		select f.id, f.owner_id, f.member_status,
		       (select count(*) from family_member fm where fm.family_id = f.id)::int,
		       (select count(*) from child_profile cp where cp.family_id = f.id)::int,
		       (select count(*) from device_bind db
		        join child_profile cp on cp.id = db.child_id
		        where cp.family_id = f.id and db.status = 'active')::int,
		       coalesce(am.status, ''), coalesce(am.plan_name, ''), am.expires_at,
		       f.created_at
		from family f
		left join account_membership am on am.account_id = f.owner_id `+filter+`
		order by f.created_at desc limit $2 offset $3`, query, limit, offset)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()
	items := make([]Summary, 0, limit)
	for rows.Next() {
		var item Summary
		if err := rows.Scan(&item.FamilyID, &item.OwnerID, &item.MemberStatus,
			&item.MembersCount, &item.ChildrenCount, &item.DevicesCount,
			&item.MembershipStatus, &item.PlanName, &item.ExpiresAt,
			&item.CreatedAt); err != nil {
			return ListResult{}, err
		}
		items = append(items, item)
	}
	return ListResult{Items: items, Total: total}, rows.Err()
}

func (s *Service) Get(ctx context.Context, id string) (Detail, error) {
	var detail Detail
	err := s.pool.QueryRow(ctx, `
		select f.id, f.owner_id, f.member_status,
		       (select count(*) from family_member fm where fm.family_id = f.id)::int,
		       (select count(*) from child_profile cp where cp.family_id = f.id)::int,
		       (select count(*) from device_bind db
		        join child_profile cp on cp.id = db.child_id
		        where cp.family_id = f.id and db.status = 'active')::int,
		       coalesce(am.status, ''), coalesce(am.plan_name, ''), am.expires_at,
		       f.created_at
		from family f
		left join account_membership am on am.account_id = f.owner_id
		where f.id = $1`, id).Scan(
		&detail.FamilyID, &detail.OwnerID, &detail.MemberStatus,
		&detail.MembersCount, &detail.ChildrenCount, &detail.DevicesCount,
		&detail.MembershipStatus, &detail.PlanName, &detail.ExpiresAt,
		&detail.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, apiError{404, "FAMILY_NOT_FOUND", "家庭不存在"}
	}
	if err != nil {
		return Detail{}, err
	}

	detail.MemberList = make([]Member, 0)
	rows, err := s.pool.Query(ctx, `
		select account_id, role, joined_at from family_member
		where family_id = $1 order by joined_at`, id)
	if err != nil {
		return Detail{}, err
	}
	for rows.Next() {
		var item Member
		if err := rows.Scan(&item.AccountID, &item.Role, &item.JoinedAt); err != nil {
			rows.Close()
			return Detail{}, err
		}
		detail.MemberList = append(detail.MemberList, item)
	}
	rows.Close()

	detail.ChildList = make([]Child, 0)
	rows, err = s.pool.Query(ctx, `
		select id, nickname, stage_code, coalesce(grade_tag, ''), created_at
		from child_profile where family_id = $1 order by created_at`, id)
	if err != nil {
		return Detail{}, err
	}
	for rows.Next() {
		var item Child
		if err := rows.Scan(&item.ID, &item.Nickname, &item.Stage, &item.GradeTag,
			&item.CreatedAt); err != nil {
			rows.Close()
			return Detail{}, err
		}
		detail.ChildList = append(detail.ChildList, item)
	}
	rows.Close()

	detail.DeviceList = make([]Device, 0)
	rows, err = s.pool.Query(ctx, `
		select db.device_id, db.platform, db.status, db.child_id, cp.nickname,
		       db.bound_at, db.token_expires_at
		from device_bind db join child_profile cp on cp.id = db.child_id
		where cp.family_id = $1 order by db.bound_at desc`, id)
	if err != nil {
		return Detail{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Device
		if err := rows.Scan(&item.DeviceID, &item.Platform, &item.Status,
			&item.ChildID, &item.ChildName, &item.BoundAt, &item.TokenExpiresAt); err != nil {
			return Detail{}, err
		}
		detail.DeviceList = append(detail.DeviceList, item)
	}
	return detail, rows.Err()
}

type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e apiError) Error() string { return e.Message }

func writeError(w http.ResponseWriter, err error) {
	var e apiError
	if errors.As(err, &e) {
		writeJSON(w, e.Status, map[string]string{"code": e.Code, "message": e.Message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "INTERNAL", "message": "服务端错误"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
