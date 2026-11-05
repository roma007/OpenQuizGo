package membership

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/admin"
	"github.com/roma007/OpenQuizGo/internal/identity"
)

type Plan struct {
	ID               string    `json:"id"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	DurationDays     int       `json:"duration_days"`
	PriceCents       int       `json:"price_cents"`
	Features         []string  `json:"features"`
	EntitlementCodes []string  `json:"entitlement_codes"`
	Status           string    `json:"status"`
	Sort             int       `json:"sort"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PlanInput struct {
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	DurationDays     int      `json:"duration_days"`
	PriceCents       int      `json:"price_cents"`
	Features         []string `json:"features"`
	EntitlementCodes []string `json:"entitlement_codes"`
	Status           string   `json:"status"`
	Sort             int      `json:"sort"`
}

type Order struct {
	ID              string          `json:"id"`
	OrderNo         string          `json:"order_no"`
	AccountID       string          `json:"account_id,omitempty"`
	PlanID          string          `json:"plan_id"`
	PlanName        string          `json:"plan_name"`
	AmountCents     int             `json:"amount_cents"`
	DurationDays    int             `json:"duration_days"`
	Status          string          `json:"status"`
	Channel         string          `json:"channel"`
	CreatedAt       time.Time       `json:"created_at"`
	PaidAt          *time.Time      `json:"paid_at,omitempty"`
	RefundedAt      *time.Time      `json:"refunded_at,omitempty"`
	RefundReason    string          `json:"refund_reason,omitempty"`
	RefundedBy      string          `json:"refunded_by,omitempty"`
	ExternalTradeNo string          `json:"external_trade_no,omitempty"`
	RefundBefore    json.RawMessage `json:"refund_before,omitempty"`
	RefundAfter     json.RawMessage `json:"refund_after,omitempty"`
}

type RefundSnapshot struct {
	Status    string    `json:"status"`
	PlanName  string    `json:"plan_name"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Membership struct {
	Active        bool       `json:"active"`
	Shared        bool       `json:"shared"`
	Status        string     `json:"status"`
	PlanName      string     `json:"plan_name,omitempty"`
	StartsAt      *time.Time `json:"starts_at,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	DaysRemaining int        `json:"days_remaining"`
	Entitlements  []string   `json:"entitlements"`
}

// Sharing 是家庭会员共享设置。
type Sharing struct {
	Shared  bool `json:"shared"`
	CanEdit bool `json:"can_edit"`
}

type Center struct {
	Membership *Membership `json:"membership,omitempty"`
	Sharing    Sharing     `json:"sharing"`
	Plans      []Plan      `json:"plans"`
}

type PayResult struct {
	Order      Order      `json:"order"`
	Membership Membership `json:"membership"`
}

type Service struct {
	pool           *pgxpool.Pool
	callbackSecret string
}

func New(pool *pgxpool.Pool, callbackSecret string) *Service {
	return &Service{pool: pool, callbackSecret: callbackSecret}
}

func (s *Service) AdminRoutes(r chi.Router, guard *admin.Service) {
	r.Group(func(r chi.Router) {
		r.Use(guard.Auth)
		r.With(guard.Require("membership.read")).Get("/admin/membership/plans", s.handleAdminList)
		r.With(guard.Require("membership.write")).Post("/admin/membership/plans", s.handleAdminCreate)
		r.With(guard.Require("membership.write")).Put("/admin/membership/plans/{id}", s.handleAdminUpdate)
		r.With(guard.Require("membership.write")).Delete("/admin/membership/plans/{id}", s.handleAdminDelete)
		r.With(guard.Require("membership.read")).Get("/admin/membership/orders", s.handleAdminOrders)
		r.With(guard.Require("membership.write")).Post("/admin/membership/orders/{id}/refund", s.handleAdminRefund)
	})
}

// PublicRoutes 注册支付渠道回调（无需家长令牌，使用 HMAC 校验）。
func (s *Service) PublicRoutes(r chi.Router) {
	r.Post("/payments/callback", s.handleCallback)
}

func (s *Service) ParentRoutes(r chi.Router) {
	r.Get("/family/membership", s.handleCenter)
	r.Get("/family/membership/sharing", s.handleSharing)
	r.Put("/family/membership/sharing", s.handleSetSharing)
	r.Get("/family/membership/orders", s.handleOrders)
	r.Post("/family/membership/orders", s.handleCreateOrder)
	r.Post("/family/membership/orders/{orderID}/pay", s.handlePayOrder)
}

func (s *Service) handleSharing(w http.ResponseWriter, r *http.Request) {
	setting, err := s.Sharing(r.Context(), identity.AccountIDFrom(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, setting)
}

func (s *Service) handleSetSharing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Shared bool `json:"shared"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不是合法 JSON"})
		return
	}
	setting, err := s.SetSharing(r.Context(), identity.AccountIDFrom(r.Context()), req.Shared)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, setting)
}

func (s *Service) handleAdminList(w http.ResponseWriter, r *http.Request) {
	plans, err := s.ListPlans(r.Context(), false)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": plans})
}

func (s *Service) handleAdminCreate(w http.ResponseWriter, r *http.Request) {
	var in PlanInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	plan, err := s.CreatePlan(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, plan)
}

func (s *Service) handleAdminUpdate(w http.ResponseWriter, r *http.Request) {
	var in PlanInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	plan, err := s.UpdatePlan(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Service) handleAdminDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.DeletePlan(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, apiError{413, "PAYLOAD_TOO_LARGE", "回调体过大"})
		return
	}
	order, err := s.Callback(r.Context(), body, r.Header.Get("X-Payment-Signature"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"order_no": order.OrderNo,
		"paid":     order.Status == "paid",
	})
}

func (s *Service) handleAdminOrders(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	orders, total, err := s.AdminOrders(r.Context(), r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": orders, "total": total})
}

func (s *Service) handleAdminRefund(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	principal, _ := admin.From(r.Context())
	order, err := s.RefundOrder(r.Context(), chi.URLParam(r, "id"), req.Reason, principal.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	s.audit(r.Context(), principal.ID, order, req.Reason, clientIP(r))
	writeJSON(w, http.StatusOK, order)
}

func (s *Service) audit(ctx context.Context, actorID string, order Order, reason, ip string) {
	payload, _ := json.Marshal(map[string]any{
		"order_no":      order.OrderNo,
		"amount_cents":  order.AmountCents,
		"duration_days": order.DurationDays,
		"reason":        reason,
		"before":        order.RefundBefore,
		"after":         order.RefundAfter,
	})
	_, _ = s.pool.Exec(ctx, `
		insert into audit_log (actor_type, actor_id, action, target_type, target_id, after_data, ip)
		values ('admin', $1, 'membership.order.refund', 'membership_order', $2, $3::jsonb,
		        nullif($4, '')::inet)`,
		actorID, order.ID, string(payload), ip)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Service) handleCenter(w http.ResponseWriter, r *http.Request) {
	accountID := identity.AccountIDFrom(r.Context())
	center, err := s.Center(r.Context(), accountID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, center)
}

func (s *Service) handleOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.Orders(r.Context(), identity.AccountIDFrom(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": orders})
}

func (s *Service) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	order, err := s.CreateOrder(r.Context(), identity.AccountIDFrom(r.Context()), req.PlanID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (s *Service) handlePayOrder(w http.ResponseWriter, r *http.Request) {
	result, err := s.PayOrder(r.Context(), identity.AccountIDFrom(r.Context()), chi.URLParam(r, "orderID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) ListPlans(ctx context.Context, onlineOnly bool) ([]Plan, error) {
	query := `
		select id, code, name, description, duration_days, price_cents,
		       features::text, entitlement_codes::text, status, sort, created_at, updated_at
		from membership_plan`
	if onlineOnly {
		query += ` where status = 'online'`
	}
	query += ` order by sort, created_at`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := make([]Plan, 0)
	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, rows.Err()
}

func (s *Service) CreatePlan(ctx context.Context, in PlanInput) (Plan, error) {
	if err := validatePlan(in); err != nil {
		return Plan{}, err
	}
	features := encodeList(in.Features)
	codes := encodeList(in.EntitlementCodes)
	var id string
	err := s.pool.QueryRow(ctx, `
		insert into membership_plan (
			code, name, description, duration_days, price_cents, features, entitlement_codes, status, sort)
		values ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9)
		returning id`,
		in.Code, in.Name, in.Description, in.DurationDays, in.PriceCents,
		features, codes, in.Status, in.Sort).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return Plan{}, apiError{409, "PLAN_CODE_EXISTS", "套餐编码已存在"}
		}
		return Plan{}, err
	}
	return s.getPlan(ctx, id)
}

func (s *Service) UpdatePlan(ctx context.Context, id string, in PlanInput) (Plan, error) {
	if err := validatePlan(in); err != nil {
		return Plan{}, err
	}
	features := encodeList(in.Features)
	codes := encodeList(in.EntitlementCodes)
	tag, err := s.pool.Exec(ctx, `
		update membership_plan set code = $2, name = $3, description = $4,
		       duration_days = $5, price_cents = $6, features = $7::jsonb,
		       entitlement_codes = $8::jsonb, status = $9, sort = $10, updated_at = now()
		where id = $1`,
		id, in.Code, in.Name, in.Description, in.DurationDays, in.PriceCents,
		features, codes, in.Status, in.Sort)
	if err != nil {
		if isUniqueViolation(err) {
			return Plan{}, apiError{409, "PLAN_CODE_EXISTS", "套餐编码已存在"}
		}
		return Plan{}, err
	}
	if tag.RowsAffected() == 0 {
		return Plan{}, apiError{404, "PLAN_NOT_FOUND", "套餐不存在"}
	}
	return s.getPlan(ctx, id)
}

func (s *Service) DeletePlan(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from membership_plan where id = $1`, id)
	if err != nil {
		if isForeignKeyViolation(err) {
			return apiError{409, "PLAN_IN_USE", "套餐已有订单引用，请改为下线"}
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return apiError{404, "PLAN_NOT_FOUND", "套餐不存在"}
	}
	return nil
}

// Sharing 返回家庭会员共享设置与当前账号是否可编辑。
func (s *Service) Sharing(ctx context.Context, accountID string) (Sharing, error) {
	var ownerID string
	var shared bool
	err := s.pool.QueryRow(ctx, `
		select f.owner_id, f.membership_shared
		from parent_account pa
		join family f on f.id = pa.active_family_id
		where pa.id = $1`, accountID).Scan(&ownerID, &shared)
	if errors.Is(err, pgx.ErrNoRows) {
		return Sharing{Shared: false, CanEdit: false}, nil
	}
	if err != nil {
		return Sharing{}, err
	}
	return Sharing{Shared: shared, CanEdit: ownerID == accountID}, nil
}

// SetSharing 由家庭 owner 修改会员共享策略。
func (s *Service) SetSharing(ctx context.Context, accountID string, shared bool) (Sharing, error) {
	var ownerID string
	err := s.pool.QueryRow(ctx, `
		select f.owner_id
		from parent_account pa
		join family f on f.id = pa.active_family_id
		where pa.id = $1`, accountID).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Sharing{}, apiError{403, "FORBIDDEN", "当前账号未加入家庭"}
	}
	if err != nil {
		return Sharing{}, err
	}
	if ownerID != accountID {
		return Sharing{}, apiError{403, "FORBIDDEN", "仅家庭创建者可修改共享策略"}
	}
	if _, err := s.pool.Exec(ctx, `
		update family set membership_shared = $2, updated_at = now() where owner_id = $1`,
		accountID, shared); err != nil {
		return Sharing{}, err
	}
	return Sharing{Shared: shared, CanEdit: true}, nil
}

func (s *Service) Center(ctx context.Context, accountID string) (Center, error) {
	if accountID == "" {
		return Center{}, apiError{401, "AUTH_REQUIRED", "家长未登录"}
	}
	plans, err := s.ListPlans(ctx, true)
	if err != nil {
		return Center{}, err
	}
	member, err := s.current(ctx, accountID)
	if err != nil {
		return Center{}, err
	}
	sharing, err := s.Sharing(ctx, accountID)
	if err != nil {
		return Center{}, err
	}
	return Center{Membership: member, Sharing: sharing, Plans: plans}, nil
}

func (s *Service) Orders(ctx context.Context, accountID string) ([]Order, error) {
	rows, err := s.pool.Query(ctx, `
		select id, order_no, account_id, plan_id, plan_name, amount_cents, duration_days,
		       status, channel, created_at, paid_at, refunded_at,
		       coalesce(refund_reason, ''), coalesce(refunded_by::text, ''),
		       coalesce(external_trade_no, ''),
		       coalesce(refund_before::text, ''),
		       coalesce(refund_after::text, '')
		from membership_order where account_id = $1
		order by created_at desc limit 20`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]Order, 0)
	for rows.Next() {
		item, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, item)
	}
	return orders, rows.Err()
}

// AdminOrders 返回全部订单，可按状态筛选。
func (s *Service) AdminOrders(ctx context.Context, status string, limit, offset int) ([]Order, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := s.pool.QueryRow(ctx,
		`select count(*)::int from membership_order where ($1 = '' or status = $1)`,
		status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		select id, order_no, account_id, plan_id, plan_name, amount_cents, duration_days,
		       status, channel, created_at, paid_at, refunded_at,
		       coalesce(refund_reason, ''), coalesce(refunded_by::text, ''),
		       coalesce(external_trade_no, ''),
		       coalesce(refund_before::text, ''),
		       coalesce(refund_after::text, '')
		from membership_order
		where ($1 = '' or status = $1)
		order by created_at desc limit $2 offset $3`, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	orders := make([]Order, 0)
	for rows.Next() {
		item, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, item)
	}
	return orders, total, rows.Err()
}

func (s *Service) CreateOrder(ctx context.Context, accountID, planID string) (Order, error) {
	if accountID == "" {
		return Order{}, apiError{401, "AUTH_REQUIRED", "家长未登录"}
	}
	var plan Plan
	row := s.pool.QueryRow(ctx, `
		select id, code, name, description, duration_days, price_cents,
		       features::text, entitlement_codes::text, status, sort, created_at, updated_at
		from membership_plan where id = $1`, planID)
	plan, err := scanPlan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, apiError{404, "PLAN_NOT_FOUND", "套餐不存在"}
	}
	if err != nil {
		return Order{}, err
	}
	if plan.Status != "online" {
		return Order{}, apiError{409, "PLAN_OFFLINE", "套餐未上线"}
	}

	item, err := scanOrder(s.pool.QueryRow(ctx, `
		insert into membership_order (
			order_no, account_id, plan_id, plan_name, amount_cents, duration_days)
		values ($1, $2, $3, $4, $5, $6)
		returning id, order_no, account_id, plan_id, plan_name, amount_cents, duration_days,
		          status, channel, created_at, paid_at, refunded_at,
		          coalesce(refund_reason, ''), coalesce(refunded_by::text, ''),
		          coalesce(external_trade_no, ''),
		          coalesce(refund_before::text, ''),
		          coalesce(refund_after::text, '')`,
		orderNo(), accountID, plan.ID, plan.Name, plan.PriceCents, plan.DurationDays))
	if err != nil {
		return Order{}, err
	}
	return item, nil
}

func (s *Service) PayOrder(ctx context.Context, accountID, orderID string) (PayResult, error) {
	if accountID == "" {
		return PayResult{}, apiError{401, "AUTH_REQUIRED", "家长未登录"}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PayResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	item, err := scanOrder(tx.QueryRow(ctx, `
		select id, order_no, account_id, plan_id, plan_name, amount_cents, duration_days,
		       status, channel, created_at, paid_at, refunded_at,
		       coalesce(refund_reason, ''), coalesce(refunded_by::text, ''),
		       coalesce(external_trade_no, ''),
		       coalesce(refund_before::text, ''),
		       coalesce(refund_after::text, '')
		from membership_order
		where id = $1 and account_id = $2 for update`, orderID, accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return PayResult{}, apiError{404, "ORDER_NOT_FOUND", "订单不存在"}
	}
	if err != nil {
		return PayResult{}, err
	}
	if item.Status == "pending" {
		if err := s.activate(ctx, tx, &item, "mock", ""); err != nil {
			return PayResult{}, err
		}
	} else if item.Status == "cancelled" {
		return PayResult{}, apiError{409, "ORDER_CANCELLED", "订单已取消"}
	}
	if err := tx.Commit(ctx); err != nil {
		return PayResult{}, err
	}

	member, err := s.current(ctx, accountID)
	if err != nil {
		return PayResult{}, err
	}
	if member == nil {
		member = &Membership{Status: "none"}
	}
	return PayResult{Order: item, Membership: *member}, nil
}

// activate 在事务内将会员订单标记为已支付并延长会员。
func (s *Service) activate(ctx context.Context, tx pgx.Tx, item *Order, channel, tradeNo string) error {
	now := time.Now()
	var existingExpiry *time.Time
	_ = tx.QueryRow(ctx,
		`select expires_at from account_membership where account_id = $1 for update`,
		item.AccountID).Scan(&existingExpiry)
	base := now
	if existingExpiry != nil && existingExpiry.After(now) {
		base = *existingExpiry
	}
	expiresAt := base.AddDate(0, 0, item.DurationDays)
	if _, err := tx.Exec(ctx, `
		insert into account_membership (account_id, plan_id, plan_name, status, starts_at, expires_at, updated_at)
		values ($1, $2, $3, 'active', $4, $5, now())
		on conflict (account_id) do update set
		    plan_id = excluded.plan_id,
		    plan_name = excluded.plan_name,
		    status = 'active',
		    starts_at = case when account_membership.status = 'active'
		                     and account_membership.expires_at > now()
		                     then account_membership.starts_at else excluded.starts_at end,
		    expires_at = excluded.expires_at,
		    updated_at = now()`,
		item.AccountID, item.PlanID, item.PlanName, now, expiresAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		update membership_order
		set status = 'paid', paid_at = now(), channel = $2, external_trade_no = $3
		where id = $1`, item.ID, channel, tradeNo); err != nil {
		return err
	}
	item.Status = "paid"
	item.Channel = channel
	item.ExternalTradeNo = tradeNo
	paidAt := now
	item.PaidAt = &paidAt
	return nil
}

// Callback 处理支付渠道回调，使用 HMAC-SHA256 校验原始请求体。
func (s *Service) Callback(ctx context.Context, body []byte, signature string) (Order, error) {
	if s.callbackSecret == "" {
		return Order{}, apiError{503, "PAYMENT_DISABLED", "支付回调未启用"}
	}
	mac := hmac.New(sha256.New, []byte(s.callbackSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(strings.TrimSpace(signature)))) {
		return Order{}, apiError{401, "SIGNATURE_INVALID", "回调签名无效"}
	}
	var req struct {
		OrderNo string `json:"order_no"`
		TradeNo string `json:"trade_no"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return Order{}, apiError{422, "VALIDATION_ERROR", "回调体不合法"}
	}
	if req.OrderNo == "" || req.TradeNo == "" {
		return Order{}, apiError{422, "VALIDATION_ERROR", "缺少订单号或交易号"}
	}
	if req.Status != "paid" {
		return Order{}, apiError{422, "VALIDATION_ERROR", "暂只支持 paid 状态"}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	item, err := scanOrder(tx.QueryRow(ctx, `
		select id, order_no, account_id, plan_id, plan_name, amount_cents, duration_days,
		       status, channel, created_at, paid_at, refunded_at,
		       coalesce(refund_reason, ''), coalesce(refunded_by::text, ''),
		       coalesce(external_trade_no, ''),
		       coalesce(refund_before::text, ''),
		       coalesce(refund_after::text, '')
		from membership_order where order_no = $1 for update`, req.OrderNo))
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, apiError{404, "ORDER_NOT_FOUND", "订单不存在"}
	}
	if err != nil {
		return Order{}, err
	}
	if item.Status == "refunded" {
		return Order{}, apiError{409, "ORDER_ALREADY_REFUNDED", "订单已退款"}
	}
	if item.Status == "pending" {
		if err := s.activate(ctx, tx, &item, "callback", req.TradeNo); err != nil {
			return Order{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return item, nil
}

// RefundOrder 模拟退款，按订单时长回退会员到期时间。
func (s *Service) RefundOrder(ctx context.Context, orderID, reason, operatorID string) (Order, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 200 {
		return Order{}, apiError{422, "VALIDATION_ERROR", "退款原因需为 1–200 字"}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	item, err := scanOrder(tx.QueryRow(ctx, `
		select id, order_no, account_id, plan_id, plan_name, amount_cents, duration_days,
		       status, channel, created_at, paid_at, refunded_at,
		       coalesce(refund_reason, ''), coalesce(refunded_by::text, ''),
		       coalesce(external_trade_no, ''),
		       coalesce(refund_before::text, ''),
		       coalesce(refund_after::text, '')
		from membership_order where id = $1 for update`, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, apiError{404, "ORDER_NOT_FOUND", "订单不存在"}
	}
	if err != nil {
		return Order{}, err
	}
	if item.Status == "refunded" {
		if err := tx.Commit(ctx); err != nil {
			return Order{}, err
		}
		return item, nil
	}
	if item.Status != "paid" {
		return Order{}, apiError{409, "ORDER_NOT_REFUNDABLE", "只有已支付订单可以退款"}
	}

	var before *RefundSnapshot
	var prevStatus, prevPlanName string
	var prevExpiresAt time.Time
	err = tx.QueryRow(ctx, `
		select status, plan_name, expires_at
		from account_membership where account_id = $1 for update`,
		item.AccountID).Scan(&prevStatus, &prevPlanName, &prevExpiresAt)
	if err == nil {
		before = &RefundSnapshot{
			Status: prevStatus, PlanName: prevPlanName, ExpiresAt: prevExpiresAt,
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Order{}, err
	}

	now := time.Now()
	var after *RefundSnapshot
	if before != nil {
		newExpiry := before.ExpiresAt.AddDate(0, 0, -item.DurationDays)
		newStatus := "active"
		if !newExpiry.After(now) {
			newStatus = "expired"
		}
		if _, err := tx.Exec(ctx, `
			update account_membership set status = $2, expires_at = $3, updated_at = now()
			where account_id = $1`, item.AccountID, newStatus, newExpiry); err != nil {
			return Order{}, err
		}
		after = &RefundSnapshot{
			Status: newStatus, PlanName: before.PlanName, ExpiresAt: newExpiry,
		}
	}

	beforeJSON, err := encodeRefundSnapshot(before)
	if err != nil {
		return Order{}, err
	}
	afterJSON, err := encodeRefundSnapshot(after)
	if err != nil {
		return Order{}, err
	}
	if _, err := tx.Exec(ctx, `
		update membership_order set status = 'refunded', refunded_at = $2,
		       refund_reason = $3, refunded_by = nullif($4, '')::uuid,
		       refund_before = coalesce(refund_before, $5::jsonb),
		       refund_after = coalesce(refund_after, $6::jsonb)
		where id = $1`,
		item.ID, now, reason, operatorID, beforeJSON, afterJSON); err != nil {
		return Order{}, err
	}
	item.Status = "refunded"
	item.RefundedAt = &now
	item.RefundReason = reason
	item.RefundedBy = operatorID
	item.RefundBefore = beforeJSON
	item.RefundAfter = afterJSON
	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return item, nil
}

func encodeRefundSnapshot(snapshot *RefundSnapshot) (json.RawMessage, error) {
	if snapshot == nil {
		return nil, nil
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Service) current(ctx context.Context, accountID string) (*Membership, error) {
	member, err := s.ownMembership(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if member != nil && member.Active {
		return member, nil
	}
	shared, err := s.sharedMembership(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if shared != nil {
		return shared, nil
	}
	return member, nil
}

// ownMembership 返回账号自己的会员记录（可能已过期）。
func (s *Service) ownMembership(ctx context.Context, accountID string) (*Membership, error) {
	var member Membership
	var expiresAt time.Time
	var startsAt time.Time
	var codes string
	err := s.pool.QueryRow(ctx, `
		select coalesce(am.plan_name, ''), am.status, am.starts_at, am.expires_at,
		       coalesce(mp.entitlement_codes::text, '[]')
		from account_membership am
		left join membership_plan mp on mp.id = am.plan_id
		where am.account_id = $1`, accountID).
		Scan(&member.PlanName, &member.Status, &startsAt, &expiresAt, &codes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.fillMembership(&member, startsAt, expiresAt, codes)
	return &member, nil
}

// sharedMembership 返回家庭 owner 共享的有效会员（未开启共享或自己就是 owner 时为空）。
func (s *Service) sharedMembership(ctx context.Context, accountID string) (*Membership, error) {
	var member Membership
	var expiresAt time.Time
	var startsAt time.Time
	var codes string
	err := s.pool.QueryRow(ctx, `
		select coalesce(am.plan_name, ''), am.status, am.starts_at, am.expires_at,
		       coalesce(mp.entitlement_codes::text, '[]')
		from parent_account pa
		join family f on f.id = pa.active_family_id
		join account_membership am on am.account_id = f.owner_id
		left join membership_plan mp on mp.id = am.plan_id
		where pa.id = $1
		  and f.membership_shared
		  and f.owner_id <> $1
		  and am.status = 'active'
		  and am.expires_at > now()`, accountID).
		Scan(&member.PlanName, &member.Status, &startsAt, &expiresAt, &codes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.fillMembership(&member, startsAt, expiresAt, codes)
	member.Shared = true
	return &member, nil
}

func (s *Service) fillMembership(member *Membership, startsAt, expiresAt time.Time, codes string) {
	now := time.Now()
	member.StartsAt = &startsAt
	member.ExpiresAt = &expiresAt
	member.Entitlements = []string{}
	if expiresAt.After(now) {
		member.Active = true
		member.Status = "active"
		member.DaysRemaining = int(math.Ceil(expiresAt.Sub(now).Hours() / 24))
		_ = json.Unmarshal([]byte(codes), &member.Entitlements)
		if member.Entitlements == nil {
			member.Entitlements = []string{}
		}
	} else {
		member.Active = false
		member.Status = "expired"
		member.DaysRemaining = 0
	}
}

func (s *Service) getPlan(ctx context.Context, id string) (Plan, error) {
	row := s.pool.QueryRow(ctx, `
		select id, code, name, description, duration_days, price_cents,
		       features::text, entitlement_codes::text, status, sort, created_at, updated_at
		from membership_plan where id = $1`, id)
	plan, err := scanPlan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, apiError{404, "PLAN_NOT_FOUND", "套餐不存在"}
	}
	return plan, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanOrder(row scanner) (Order, error) {
	var item Order
	var refundBefore, refundAfter string
	err := row.Scan(&item.ID, &item.OrderNo, &item.AccountID, &item.PlanID,
		&item.PlanName, &item.AmountCents, &item.DurationDays, &item.Status,
		&item.Channel, &item.CreatedAt, &item.PaidAt, &item.RefundedAt,
		&item.RefundReason, &item.RefundedBy, &item.ExternalTradeNo,
		&refundBefore, &refundAfter)
	if err != nil {
		return item, err
	}
	if refundBefore != "" {
		item.RefundBefore = json.RawMessage(refundBefore)
	}
	if refundAfter != "" {
		item.RefundAfter = json.RawMessage(refundAfter)
	}
	return item, nil
}

func scanPlan(row scanner) (Plan, error) {
	var plan Plan
	var features, codes string
	if err := row.Scan(&plan.ID, &plan.Code, &plan.Name, &plan.Description,
		&plan.DurationDays, &plan.PriceCents, &features, &codes, &plan.Status, &plan.Sort,
		&plan.CreatedAt, &plan.UpdatedAt); err != nil {
		return Plan{}, err
	}
	_ = json.Unmarshal([]byte(features), &plan.Features)
	if plan.Features == nil {
		plan.Features = []string{}
	}
	_ = json.Unmarshal([]byte(codes), &plan.EntitlementCodes)
	if plan.EntitlementCodes == nil {
		plan.EntitlementCodes = []string{}
	}
	return plan, nil
}

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)

func validatePlan(in PlanInput) error {
	if !codePattern.MatchString(in.Code) {
		return apiError{422, "VALIDATION_ERROR", "编码需为 2–32 位小写字母/数字/连字符"}
	}
	if in.Name == "" || len([]rune(in.Name)) > 50 {
		return apiError{422, "VALIDATION_ERROR", "名称需为 1–50 字"}
	}
	if in.DurationDays < 1 || in.DurationDays > 3650 {
		return apiError{422, "VALIDATION_ERROR", "时长需在 1–3650 天"}
	}
	if in.PriceCents < 0 || in.PriceCents > 10_000_000 {
		return apiError{422, "VALIDATION_ERROR", "价格不合法"}
	}
	statuses := map[string]bool{"draft": true, "online": true, "offline": true}
	if !statuses[in.Status] {
		return apiError{422, "VALIDATION_ERROR", "状态不合法"}
	}
	allowedEntitlements := map[string]bool{
		"topic_pack": true, "report_advanced": true, "customer_service": true,
	}
	if len(in.EntitlementCodes) > 10 {
		return apiError{422, "VALIDATION_ERROR", "功能权益最多 10 项"}
	}
	seen := map[string]bool{}
	for _, code := range in.EntitlementCodes {
		if !allowedEntitlements[code] {
			return apiError{422, "VALIDATION_ERROR", "不支持的功能权益：" + code}
		}
		if seen[code] {
			return apiError{422, "VALIDATION_ERROR", "功能权益重复：" + code}
		}
		seen[code] = true
	}
	if len(in.Features) > 20 {
		return apiError{422, "VALIDATION_ERROR", "权益最多 20 条"}
	}
	for _, item := range in.Features {
		if len([]rune(item)) > 100 {
			return apiError{422, "VALIDATION_ERROR", "单条权益最多 100 字"}
		}
	}
	if in.Sort < 0 || in.Sort > 10000 {
		return apiError{422, "VALIDATION_ERROR", "排序值不合法"}
	}
	return nil
}

func encodeList(items []string) string {
	if items == nil {
		items = []string{}
	}
	encoded, _ := json.Marshal(items)
	return string(encoded)
}

func orderNo() string {
	return fmt.Sprintf("M%s%06d", time.Now().Format("20060102150405"), rand.Intn(1000000))
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
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
