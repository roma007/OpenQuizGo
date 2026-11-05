package experiment

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/admin"
	"github.com/roma007/OpenQuizGo/internal/identity"
)

type Experiment struct {
	ID             string    `json:"id"`
	Key            string    `json:"key"`
	Name           string    `json:"name"`
	VariantA       string    `json:"variant_a"`
	VariantB       string    `json:"variant_b"`
	TrafficPercent int       `json:"traffic_percent"`
	Status         string    `json:"status"`
	ExposureA      int       `json:"exposure_a"`
	ExposureB      int       `json:"exposure_b"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Input struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	VariantA       string `json:"variant_a"`
	VariantB       string `json:"variant_b"`
	TrafficPercent int    `json:"traffic_percent"`
	Status         string `json:"status"`
}

type Assignment struct {
	Key            string `json:"key"`
	Variant        string `json:"variant"`
	TrafficPercent int    `json:"traffic_percent"`
}

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

var keyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

func (s *Service) AdminRoutes(r chi.Router, guard *admin.Service) {
	r.Group(func(r chi.Router) {
		r.Use(guard.Auth)
		r.With(guard.Require("experiment.read")).Get("/admin/experiments", s.handleList)
		r.With(guard.Require("experiment.write")).Post("/admin/experiments", s.handleCreate)
		r.With(guard.Require("experiment.write")).Put("/admin/experiments/{id}", s.handleUpdate)
	})
}

func (s *Service) ParentRoutes(r chi.Router) {
	r.Get("/family/experiments/{key}", s.handleAssign)
}

const columns = `e.id, e.key, e.name, e.variant_a, e.variant_b, e.traffic_percent,
	e.status, e.created_at, e.updated_at,
	count(ee.*) filter (where ee.variant = e.variant_a)::int,
	count(ee.*) filter (where ee.variant = e.variant_b)::int`

func scan(row pgx.Row) (Experiment, error) {
	var item Experiment
	err := row.Scan(&item.ID, &item.Key, &item.Name, &item.VariantA, &item.VariantB,
		&item.TrafficPercent, &item.Status, &item.CreatedAt, &item.UpdatedAt,
		&item.ExposureA, &item.ExposureB)
	return item, err
}

func (s *Service) List(ctx context.Context) ([]Experiment, error) {
	rows, err := s.pool.Query(ctx, `
		select `+columns+`
		from experiment e left join experiment_exposure ee on ee.experiment_id = e.id
		group by e.id order by e.created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Experiment, 0)
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func validate(in Input) error {
	if !keyPattern.MatchString(strings.TrimSpace(in.Key)) {
		return &Error{422, "VALIDATION_ERROR", "实验 key 需为 2–64 位小写字母/数字/连字符"}
	}
	if strings.TrimSpace(in.Name) == "" || len([]rune(in.Name)) > 100 {
		return &Error{422, "VALIDATION_ERROR", "名称需为 1–100 字"}
	}
	if strings.TrimSpace(in.VariantA) == "" || strings.TrimSpace(in.VariantB) == "" ||
		in.VariantA == in.VariantB {
		return &Error{422, "VALIDATION_ERROR", "变体名称需非空且不同"}
	}
	if in.TrafficPercent < 1 || in.TrafficPercent > 100 {
		return &Error{422, "VALIDATION_ERROR", "流量需在 1–100"}
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if in.Status != "active" && in.Status != "disabled" {
		return &Error{422, "VALIDATION_ERROR", "状态不合法"}
	}
	return nil
}

func (s *Service) Create(ctx context.Context, in Input) (Experiment, error) {
	if err := validate(in); err != nil {
		return Experiment{}, err
	}
	item, err := scan(s.pool.QueryRow(ctx, `
		insert into experiment (key, name, variant_a, variant_b, traffic_percent, status)
		values ($1, $2, $3, $4, $5, $6)
		returning id, key, name, variant_a, variant_b, traffic_percent, status,
		          created_at, updated_at, 0, 0`,
		strings.TrimSpace(in.Key), strings.TrimSpace(in.Name), in.VariantA,
		in.VariantB, in.TrafficPercent, in.Status))
	if err != nil {
		if isUnique(err) {
			return Experiment{}, &Error{409, "EXPERIMENT_EXISTS", "实验 key 已存在"}
		}
		return Experiment{}, err
	}
	return item, nil
}

func (s *Service) Update(ctx context.Context, id string, in Input) (Experiment, error) {
	if err := validate(in); err != nil {
		return Experiment{}, err
	}
	item, err := scan(s.pool.QueryRow(ctx, `
		update experiment set key = $2, name = $3, variant_a = $4, variant_b = $5,
		       traffic_percent = $6, status = $7, updated_at = now()
		where id = $1
		returning id, key, name, variant_a, variant_b, traffic_percent, status,
		          created_at, updated_at, 0, 0`,
		id, strings.TrimSpace(in.Key), strings.TrimSpace(in.Name), in.VariantA,
		in.VariantB, in.TrafficPercent, in.Status))
	if errors.Is(err, pgx.ErrNoRows) {
		return Experiment{}, &Error{404, "EXPERIMENT_NOT_FOUND", "实验不存在"}
	}
	if err != nil {
		if isUnique(err) {
			return Experiment{}, &Error{409, "EXPERIMENT_EXISTS", "实验 key 已存在"}
		}
		return Experiment{}, err
	}
	return item, nil
}

// VariantFor 根据 key 与账号稳定分流。
func VariantFor(key, accountID string, trafficPercent int) string {
	sum := sha256.Sum256([]byte(key + ":" + accountID))
	bucket := int(binary.BigEndian.Uint32(sum[:4]) % 100)
	if bucket < trafficPercent {
		return "treatment"
	}
	return "control"
}

// Assign 返回账号变体并记录首次曝光。
func (s *Service) Assign(ctx context.Context, accountID, key string) (Assignment, error) {
	if accountID == "" {
		return Assignment{}, &Error{401, "AUTH_REQUIRED", "家长未登录"}
	}
	var id, variantA, variantB, status string
	var traffic int
	err := s.pool.QueryRow(ctx, `
		select id, variant_a, variant_b, traffic_percent, status
		from experiment where key = $1`, strings.TrimSpace(key)).
		Scan(&id, &variantA, &variantB, &traffic, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Assignment{}, &Error{404, "EXPERIMENT_NOT_FOUND", "实验不存在"}
	}
	if err != nil {
		return Assignment{}, err
	}
	if status != "active" {
		return Assignment{}, &Error{404, "EXPERIMENT_NOT_FOUND", "实验未启用"}
	}
	variant := variantA
	if VariantFor(key, accountID, traffic) == "treatment" {
		variant = variantB
	}
	if _, err := s.pool.Exec(ctx, `
		insert into experiment_exposure (experiment_id, account_id, variant)
		values ($1, $2, $3) on conflict (experiment_id, account_id) do nothing`,
		id, accountID, variant); err != nil {
		return Assignment{}, err
	}
	return Assignment{Key: key, Variant: variant, TrafficPercent: traffic}, nil
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	items, err := s.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	var in Input
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, &Error{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	item, err := s.Create(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Service) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var in Input
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, &Error{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	item, err := s.Update(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Service) handleAssign(w http.ResponseWriter, r *http.Request) {
	result, err := s.Assign(r.Context(), identity.AccountIDFrom(r.Context()),
		chi.URLParam(r, "key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func isUnique(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}

func writeError(w http.ResponseWriter, err error) {
	var e *Error
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
