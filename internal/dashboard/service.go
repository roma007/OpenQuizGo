package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/admin"
)

type QuestionStats struct {
	Total  int `json:"total"`
	Online int `json:"online"`
	Draft  int `json:"draft"`
	Review int `json:"review"`
}

type TopicStats struct {
	Total  int `json:"total"`
	Online int `json:"online"`
	Active int `json:"active"`
}

type MembershipStats struct {
	ActiveMembers int `json:"active_members"`
	RevenueCents  int `json:"revenue_cents"`
	Orders30d     int `json:"orders_30d"`
	Paid30d       int `json:"paid_30d"`
}

type OrderSummary struct {
	OrderNo     string    `json:"order_no"`
	PlanName    string    `json:"plan_name"`
	AmountCents int       `json:"amount_cents"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type ExpiringMember struct {
	AccountID     string    `json:"account_id"`
	PlanName      string    `json:"plan_name"`
	ExpiresAt     time.Time `json:"expires_at"`
	DaysRemaining int       `json:"days_remaining"`
}

type TrendPoint struct {
	Date         string `json:"date"`
	Orders       int    `json:"orders"`
	RevenueCents int    `json:"revenue_cents"`
}

type Overview struct {
	Questions           QuestionStats    `json:"questions"`
	Topics              TopicStats       `json:"topics"`
	Membership          MembershipStats  `json:"membership"`
	Families            int              `json:"families"`
	Children            int              `json:"children"`
	Battles7d           int              `json:"battles_7d"`
	RecentOrders        []OrderSummary   `json:"recent_orders"`
	ExpiringMemberships []ExpiringMember `json:"expiring_memberships"`
	Trend               []TrendPoint     `json:"trend"`
	GeneratedAt         time.Time        `json:"generated_at"`
}

// GrowthPoint 是单日增长指标。
type GrowthPoint struct {
	Date           string `json:"date"`
	NewAccounts    int    `json:"new_accounts"`
	ActiveAccounts int    `json:"active_accounts"`
	ActiveChildren int    `json:"active_children"`
	PaidOrders     int    `json:"paid_orders"`
}

// GrowthFunnel 是转化漏斗。
type GrowthFunnel struct {
	Registered int `json:"registered"`
	WithChild  int `json:"with_child"`
	WithDevice int `json:"with_device"`
	Watched    int `json:"watched"`
	Ordered    int `json:"ordered"`
}

// RetentionRow 是注册 cohort 留存。
type RetentionRow struct {
	CohortDate  string `json:"cohort_date"`
	NewAccounts int    `json:"new_accounts"`
	D1          int    `json:"d1"`
	D7          int    `json:"d7"`
}

// GrowthReport 是增长/留存报表。
type GrowthReport struct {
	Days      int            `json:"days"`
	Daily     []GrowthPoint  `json:"daily"`
	Funnel    GrowthFunnel   `json:"funnel"`
	Retention []RetentionRow `json:"retention"`
}

type cacheEntry struct {
	overview Overview
	cachedAt time.Time
}

type Service struct {
	pool *pgxpool.Pool

	mu       sync.Mutex
	cache    map[int]cacheEntry
	cacheTTL time.Duration
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{
		pool:     pool,
		cache:    make(map[int]cacheEntry),
		cacheTTL: 30 * time.Second,
	}
}

func (s *Service) Routes(r chi.Router, guard *admin.Service) {
	r.Group(func(r chi.Router) {
		r.Use(guard.Auth)
		r.With(guard.Require("dashboard.read")).Get("/admin/dashboard", s.handleOverview)
		r.With(guard.Require("dashboard.read")).Get("/admin/analytics/growth", s.handleGrowth)
	})
}

func (s *Service) handleOverview(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days == 0 {
		days = 7
	}
	if days != 7 && days != 30 && days != 90 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"code": "VALIDATION_ERROR", "message": "days 仅支持 7/30/90",
		})
		return
	}
	overview, err := s.OverviewRange(r.Context(), days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"code": "INTERNAL", "message": "服务端错误",
		})
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

// Overview 返回默认 7 天趋势。
func (s *Service) handleGrowth(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	report, err := s.Growth(r.Context(), days)
	if err != nil {
		code := "INTERNAL"
		status := http.StatusInternalServerError
		if err == errInvalidDays {
			code = "VALIDATION_ERROR"
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, map[string]string{"code": code, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

var errInvalidDays = errors.New("days 需在 1–90")

// Growth 返回增长、漏斗与留存数据。
func (s *Service) Growth(ctx context.Context, days int) (GrowthReport, error) {
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		return GrowthReport{}, errInvalidDays
	}
	end := time.Now()
	start := end.AddDate(0, 0, -(days - 1))
	cohortStart := end.AddDate(0, 0, -13)
	report := GrowthReport{Days: days, Daily: make([]GrowthPoint, 0), Retention: make([]RetentionRow, 0)}

	rows, err := s.pool.Query(ctx, `
		select to_char(d, 'YYYY-MM-DD'),
		       (select count(*)::int from parent_account pa where pa.created_at::date = d),
		       (select count(distinct ps.account_id)::int from parent_session ps where ps.created_at::date = d),
		       (select count(distinct wl.child_id)::int from watch_log wl where wl.start_at::date = d),
		       (select count(*)::int from membership_order mo where mo.paid_at::date = d)
		from generate_series($1::date, $2::date, interval '1 day') d
		order by d`, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return GrowthReport{}, err
	}
	for rows.Next() {
		var point GrowthPoint
		if err := rows.Scan(&point.Date, &point.NewAccounts, &point.ActiveAccounts,
			&point.ActiveChildren, &point.PaidOrders); err != nil {
			rows.Close()
			return GrowthReport{}, err
		}
		report.Daily = append(report.Daily, point)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return GrowthReport{}, err
	}

	if err := s.pool.QueryRow(ctx, `
		select
		  (select count(*)::int from parent_account),
		  (select count(distinct fm.account_id)::int
		   from family_member fm join child_profile cp on cp.family_id = fm.family_id),
		  (select count(distinct fm.account_id)::int
		   from family_member fm
		   join child_profile cp on cp.family_id = fm.family_id
		   join device_bind db on db.child_id = cp.id and db.status = 'active'),
		  (select count(distinct fm.account_id)::int
		   from family_member fm
		   join child_profile cp on cp.family_id = fm.family_id
		   join watch_log wl on wl.child_id = cp.id),
		  (select count(distinct account_id)::int from membership_order
		   where status in ('paid', 'refunded'))`).
		Scan(&report.Funnel.Registered, &report.Funnel.WithChild,
			&report.Funnel.WithDevice, &report.Funnel.Watched, &report.Funnel.Ordered); err != nil {
		return GrowthReport{}, err
	}

	cohorts, err := s.pool.Query(ctx, `
		with cohorts as (
		  select id, created_at::date d from parent_account
		  where created_at::date >= $1::date
		)
		select to_char(c.d, 'YYYY-MM-DD'), count(*)::int,
		       count(*) filter (where exists (
		           select 1 from parent_session ps
		           where ps.account_id = c.id and ps.created_at::date = c.d + 1))::int,
		       count(*) filter (where exists (
		           select 1 from parent_session ps
		           where ps.account_id = c.id and ps.created_at::date = c.d + 7))::int
		from cohorts c group by c.d order by c.d`,
		cohortStart.Format("2006-01-02"))
	if err != nil {
		return GrowthReport{}, err
	}
	defer cohorts.Close()
	for cohorts.Next() {
		var row RetentionRow
		if err := cohorts.Scan(&row.CohortDate, &row.NewAccounts, &row.D1, &row.D7); err != nil {
			return GrowthReport{}, err
		}
		report.Retention = append(report.Retention, row)
	}
	return report, cohorts.Err()
}

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	return s.OverviewRange(ctx, 7)
}

// OverviewRange 聚合管理端运营指标，按天数缓存 30 秒。
func (s *Service) OverviewRange(ctx context.Context, days int) (Overview, error) {
	if days != 7 && days != 30 && days != 90 {
		days = 7
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.cache[days]
	if ok && time.Since(entry.cachedAt) < s.cacheTTL {
		return entry.overview, nil
	}

	out, err := s.loadOverview(ctx, days)
	if err != nil {
		return Overview{}, err
	}
	s.cache[days] = cacheEntry{overview: out, cachedAt: time.Now()}
	return out, nil
}

func (s *Service) loadOverview(ctx context.Context, days int) (Overview, error) {
	var out Overview
	out.GeneratedAt = time.Now()

	if err := s.pool.QueryRow(ctx, `
		select count(*)::int,
		       coalesce(sum(case when status = 'online' then 1 else 0 end), 0)::int,
		       coalesce(sum(case when status = 'draft' then 1 else 0 end), 0)::int,
		       coalesce(sum(case when status = 'review' then 1 else 0 end), 0)::int
		from question`).Scan(
		&out.Questions.Total, &out.Questions.Online,
		&out.Questions.Draft, &out.Questions.Review); err != nil {
		return Overview{}, err
	}

	if err := s.pool.QueryRow(ctx, `
		select count(*)::int,
		       coalesce(sum(case when status = 'online' then 1 else 0 end), 0)::int,
		       coalesce(sum(case when status = 'online'
		                          and (start_at is null or start_at <= now())
		                          and (end_at is null or end_at >= now())
		                         then 1 else 0 end), 0)::int
		from topic_pack`).Scan(
		&out.Topics.Total, &out.Topics.Online, &out.Topics.Active); err != nil {
		return Overview{}, err
	}

	if err := s.pool.QueryRow(ctx, `
		select
		    (select count(*)::int from account_membership
		     where status = 'active' and expires_at > now()),
		    (select coalesce(sum(case
		                when status = 'paid' then amount_cents
		                when status = 'refunded' then -amount_cents
		                else 0 end), 0)::int
		     from membership_order),
		    (select count(*)::int from membership_order
		     where created_at > now() - interval '30 days'),
		    (select count(*)::int from membership_order
		     where status = 'paid' and paid_at > now() - interval '30 days')`).Scan(
		&out.Membership.ActiveMembers, &out.Membership.RevenueCents,
		&out.Membership.Orders30d, &out.Membership.Paid30d); err != nil {
		return Overview{}, err
	}

	if err := s.pool.QueryRow(ctx, `
		select
		    (select count(*)::int from family),
		    (select count(*)::int from child_profile),
		    (select count(*)::int from family_battle
		     where status = 'finished' and finished_at > now() - interval '7 days')`).Scan(
		&out.Families, &out.Children, &out.Battles7d); err != nil {
		return Overview{}, err
	}

	rows, err := s.pool.Query(ctx, `
		select order_no, plan_name, amount_cents, status, created_at
		from membership_order order by created_at desc limit 5`)
	if err != nil {
		return Overview{}, err
	}
	defer rows.Close()
	out.RecentOrders = make([]OrderSummary, 0, 5)
	for rows.Next() {
		var item OrderSummary
		if err := rows.Scan(&item.OrderNo, &item.PlanName, &item.AmountCents,
			&item.Status, &item.CreatedAt); err != nil {
			return Overview{}, err
		}
		out.RecentOrders = append(out.RecentOrders, item)
	}
	if err := rows.Err(); err != nil {
		return Overview{}, err
	}

	expiringRows, err := s.pool.Query(ctx, `
		select account_id, plan_name, expires_at,
		       greatest(0, ceil(extract(epoch from expires_at - now()) / 86400))::int
		from account_membership
		where status = 'active' and expires_at > now()
		  and expires_at <= now() + interval '7 days'
		order by expires_at limit 5`)
	if err != nil {
		return Overview{}, err
	}
	defer expiringRows.Close()
	out.ExpiringMemberships = make([]ExpiringMember, 0, 5)
	for expiringRows.Next() {
		var item ExpiringMember
		if err := expiringRows.Scan(&item.AccountID, &item.PlanName,
			&item.ExpiresAt, &item.DaysRemaining); err != nil {
			return Overview{}, err
		}
		out.ExpiringMemberships = append(out.ExpiringMemberships, item)
	}
	if err := expiringRows.Err(); err != nil {
		return Overview{}, err
	}

	trendRows, err := s.pool.Query(ctx, `
		with days as (
		    select generate_series(
		               current_date - (($1 - 1) * interval '1 day'),
		               current_date, interval '1 day')::date as day
		)
		select to_char(d.day, 'MM-DD'),
		       count(o.id)::int,
		       coalesce(sum(case when o.status = 'paid' then o.amount_cents else 0 end), 0)::int
		from days d
		left join membership_order o
		       on o.created_at >= d.day
		      and o.created_at < d.day + interval '1 day'
		group by d.day
		order by d.day`, days)
	if err != nil {
		return Overview{}, err
	}
	defer trendRows.Close()
	out.Trend = make([]TrendPoint, 0, days)
	for trendRows.Next() {
		var item TrendPoint
		if err := trendRows.Scan(&item.Date, &item.Orders, &item.RevenueCents); err != nil {
			return Overview{}, err
		}
		out.Trend = append(out.Trend, item)
	}
	return out, trendRows.Err()
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
