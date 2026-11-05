package membership

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/csv"
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
	ID                  string          `json:"id"`
	OrderNo             string          `json:"order_no"`
	AccountID           string          `json:"account_id,omitempty"`
	PlanID              string          `json:"plan_id"`
	PlanName            string          `json:"plan_name"`
	AmountCents         int             `json:"amount_cents"`
	OriginalAmountCents int             `json:"original_amount_cents"`
	DiscountCents       int             `json:"discount_cents"`
	CouponCode          string          `json:"coupon_code,omitempty"`
	DurationDays        int             `json:"duration_days"`
	Status              string          `json:"status"`
	Channel             string          `json:"channel"`
	CreatedAt           time.Time       `json:"created_at"`
	PaidAt              *time.Time      `json:"paid_at,omitempty"`
	RefundedAt          *time.Time      `json:"refunded_at,omitempty"`
	RefundReason        string          `json:"refund_reason,omitempty"`
	RefundedBy          string          `json:"refunded_by,omitempty"`
	ExternalTradeNo     string          `json:"external_trade_no,omitempty"`
	RefundBefore        json.RawMessage `json:"refund_before,omitempty"`
	RefundAfter         json.RawMessage `json:"refund_after,omitempty"`
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

// Coupon 是会员优惠券。
type Coupon struct {
	ID             string     `json:"id"`
	Code           string     `json:"code"`
	Name           string     `json:"name"`
	AmountCents    int        `json:"amount_cents"`
	MinAmountCents int        `json:"min_amount_cents"`
	StartsAt       *time.Time `json:"starts_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	MaxUses        int        `json:"max_uses"`
	UsedCount      int        `json:"used_count"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CouponInput 是创建/更新优惠券的入参。
type CouponInput struct {
	Code           string     `json:"code"`
	Name           string     `json:"name"`
	AmountCents    int        `json:"amount_cents"`
	MinAmountCents int        `json:"min_amount_cents"`
	StartsAt       *time.Time `json:"starts_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	MaxUses        int        `json:"max_uses"`
	Status         string     `json:"status"`
}

// CouponQuote 是优惠券试算结果。
type CouponQuote struct {
	Code          string `json:"code"`
	DiscountCents int    `json:"discount_cents"`
	PayableCents  int    `json:"payable_cents"`
}

// ReconciliationDay 是单日对账汇总。
type ReconciliationDay struct {
	Date          string `json:"date"`
	PaidCount     int    `json:"paid_count"`
	PaidCents     int    `json:"paid_cents"`
	RefundedCount int    `json:"refunded_count"`
	RefundedCents int    `json:"refunded_cents"`
	DiscountCents int    `json:"discount_cents"`
	NetCents      int    `json:"net_cents"`
}

// ReconciliationReport 是会员对账报表。
type ReconciliationReport struct {
	From   string              `json:"from"`
	To     string              `json:"to"`
	Days   []ReconciliationDay `json:"days"`
	Totals ReconciliationDay   `json:"totals"`
}

// Invoice 是会员发票申请。
type Invoice struct {
	ID          string     `json:"id"`
	OrderID     string     `json:"order_id"`
	AccountID   string     `json:"account_id,omitempty"`
	TitleType   string     `json:"title_type"`
	Title       string     `json:"title"`
	TaxNo       string     `json:"tax_no,omitempty"`
	Email       string     `json:"email,omitempty"`
	AmountCents int        `json:"amount_cents"`
	Status      string     `json:"status"`
	Remark      string     `json:"remark,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	IssuedAt    *time.Time `json:"issued_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// InvoiceInput 是发票申请入参。
type InvoiceInput struct {
	TitleType string `json:"title_type"`
	Title     string `json:"title"`
	TaxNo     string `json:"tax_no"`
	Email     string `json:"email"`
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
		r.With(guard.Require("membership.read")).Get("/admin/membership/coupons", s.handleAdminCoupons)
		r.With(guard.Require("membership.write")).Post("/admin/membership/coupons", s.handleAdminCreateCoupon)
		r.With(guard.Require("membership.write")).Put("/admin/membership/coupons/{id}", s.handleAdminUpdateCoupon)
		r.With(guard.Require("membership.read")).Get("/admin/membership/reconciliation", s.handleReconciliation)
		r.With(guard.Require("membership.read")).Get("/admin/membership/reconciliation/export", s.handleReconciliationExport)
		r.With(guard.Require("membership.read")).Get("/admin/membership/invoices", s.handleAdminInvoices)
		r.With(guard.Require("membership.write")).Put("/admin/membership/invoices/{id}", s.handleAdminUpdateInvoice)
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
	r.Post("/family/membership/coupons/validate", s.handleValidateCoupon)
	r.Get("/family/membership/invoices", s.handleInvoices)
	r.Post("/family/membership/orders/{orderID}/invoice", s.handleCreateInvoice)
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

func (s *Service) handleAdminCoupons(w http.ResponseWriter, r *http.Request) {
	items, err := s.ListCoupons(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) handleAdminCreateCoupon(w http.ResponseWriter, r *http.Request) {
	var in CouponInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	coupon, err := s.CreateCoupon(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, coupon)
}

func (s *Service) handleAdminUpdateCoupon(w http.ResponseWriter, r *http.Request) {
	var in CouponInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	coupon, err := s.UpdateCoupon(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, coupon)
}

func (s *Service) handleValidateCoupon(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code   string `json:"code"`
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	quote, err := s.ValidateCoupon(r.Context(), req.Code, req.PlanID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, quote)
}

func (s *Service) handleInvoices(w http.ResponseWriter, r *http.Request) {
	items, err := s.Invoices(r.Context(), identity.AccountIDFrom(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) handleCreateInvoice(w http.ResponseWriter, r *http.Request) {
	var in InvoiceInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	invoice, err := s.CreateInvoice(
		r.Context(), identity.AccountIDFrom(r.Context()),
		chi.URLParam(r, "orderID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, invoice)
}

func (s *Service) handleAdminInvoices(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.AdminInvoices(r.Context(), r.URL.Query().Get("status"), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) handleAdminUpdateInvoice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status string `json:"status"`
		Remark string `json:"remark"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	invoice, err := s.UpdateInvoiceStatus(
		r.Context(), chi.URLParam(r, "id"), req.Status, req.Remark)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, invoice)
}

func (s *Service) handleReconciliation(w http.ResponseWriter, r *http.Request) {
	report, err := s.Reconciliation(r.Context(),
		r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Service) handleReconciliationExport(w http.ResponseWriter, r *http.Request) {
	report, err := s.Reconciliation(r.Context(),
		r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="membership-reconciliation.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"date", "paid_count", "paid_cents",
		"refunded_count", "refunded_cents", "discount_cents", "net_cents"})
	for _, day := range report.Days {
		_ = writer.Write([]string{day.Date,
			strconv.Itoa(day.PaidCount), strconv.Itoa(day.PaidCents),
			strconv.Itoa(day.RefundedCount), strconv.Itoa(day.RefundedCents),
			strconv.Itoa(day.DiscountCents), strconv.Itoa(day.NetCents)})
	}
	writer.Flush()
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
		PlanID     string `json:"plan_id"`
		CouponCode string `json:"coupon_code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	order, err := s.CreateOrderWithCoupon(
		r.Context(), identity.AccountIDFrom(r.Context()), req.PlanID, req.CouponCode)
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
		select id, order_no, account_id, plan_id, plan_name, amount_cents, original_amount_cents, discount_cents, coupon_code, duration_days,
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
		select id, order_no, account_id, plan_id, plan_name, amount_cents, original_amount_cents, discount_cents, coupon_code, duration_days,
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

var couponCodePattern = regexp.MustCompile(`^[A-Z0-9]{4,32}$`)

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func normalizeCouponCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func validateCouponInput(in CouponInput) error {
	if !couponCodePattern.MatchString(normalizeCouponCode(in.Code)) {
		return apiError{422, "VALIDATION_ERROR", "优惠码需为 4–32 位大写字母或数字"}
	}
	if strings.TrimSpace(in.Name) == "" || len([]rune(in.Name)) > 100 {
		return apiError{422, "VALIDATION_ERROR", "名称需为 1–100 字"}
	}
	if in.AmountCents <= 0 || in.MinAmountCents < 0 || in.MaxUses < 0 {
		return apiError{422, "VALIDATION_ERROR", "优惠金额/门槛/次数不合法"}
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if in.Status != "active" && in.Status != "disabled" {
		return apiError{422, "VALIDATION_ERROR", "状态不合法"}
	}
	if in.StartsAt != nil && in.ExpiresAt != nil && !in.ExpiresAt.After(*in.StartsAt) {
		return apiError{422, "VALIDATION_ERROR", "结束时间需晚于开始时间"}
	}
	return nil
}

const couponColumns = `id, code, name, amount_cents, min_amount_cents, starts_at, expires_at,
	max_uses, used_count, status, created_at, updated_at`

func scanCoupon(row scanner) (Coupon, error) {
	var item Coupon
	err := row.Scan(&item.ID, &item.Code, &item.Name, &item.AmountCents,
		&item.MinAmountCents, &item.StartsAt, &item.ExpiresAt, &item.MaxUses,
		&item.UsedCount, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

// ListCoupons 返回全部优惠券。
func (s *Service) ListCoupons(ctx context.Context) ([]Coupon, error) {
	rows, err := s.pool.Query(ctx, `select `+couponColumns+` from membership_coupon order by created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Coupon, 0)
	for rows.Next() {
		item, err := scanCoupon(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateCoupon 创建优惠券。
func (s *Service) CreateCoupon(ctx context.Context, in CouponInput) (Coupon, error) {
	if err := validateCouponInput(in); err != nil {
		return Coupon{}, err
	}
	item, err := scanCoupon(s.pool.QueryRow(ctx, `
		insert into membership_coupon (
			code, name, amount_cents, min_amount_cents, starts_at, expires_at, max_uses, status)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		returning `+couponColumns,
		normalizeCouponCode(in.Code), strings.TrimSpace(in.Name), in.AmountCents,
		in.MinAmountCents, in.StartsAt, in.ExpiresAt, in.MaxUses, in.Status))
	if err != nil {
		if isUniqueViolation(err) {
			return Coupon{}, apiError{409, "COUPON_EXISTS", "优惠码已存在"}
		}
		return Coupon{}, err
	}
	return item, nil
}

// UpdateCoupon 更新优惠券。
func (s *Service) UpdateCoupon(ctx context.Context, id string, in CouponInput) (Coupon, error) {
	if err := validateCouponInput(in); err != nil {
		return Coupon{}, err
	}
	item, err := scanCoupon(s.pool.QueryRow(ctx, `
		update membership_coupon set code = $2, name = $3, amount_cents = $4,
		       min_amount_cents = $5, starts_at = $6, expires_at = $7,
		       max_uses = $8, status = $9, updated_at = now()
		where id = $1
		returning `+couponColumns,
		id, normalizeCouponCode(in.Code), strings.TrimSpace(in.Name), in.AmountCents,
		in.MinAmountCents, in.StartsAt, in.ExpiresAt, in.MaxUses, in.Status))
	if errors.Is(err, pgx.ErrNoRows) {
		return Coupon{}, apiError{404, "COUPON_NOT_FOUND", "优惠券不存在"}
	}
	if err != nil {
		if isUniqueViolation(err) {
			return Coupon{}, apiError{409, "COUPON_EXISTS", "优惠码已存在"}
		}
		return Coupon{}, err
	}
	return item, nil
}

// couponDiscount 校验优惠券并计算优惠金额；lock=true 时对券行加锁。
func (s *Service) couponDiscount(ctx context.Context, q rowQuerier, code string,
	priceCents int, now time.Time, lock bool) (Coupon, int, error) {
	query := `select ` + couponColumns + ` from membership_coupon where code = $1`
	if lock {
		query += ` for update`
	}
	item, err := scanCoupon(q.QueryRow(ctx, query, normalizeCouponCode(code)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Coupon{}, 0, apiError{422, "COUPON_INVALID", "优惠码不存在"}
	}
	if err != nil {
		return Coupon{}, 0, err
	}
	if item.Status != "active" {
		return Coupon{}, 0, apiError{422, "COUPON_INVALID", "优惠码已停用"}
	}
	if item.StartsAt != nil && now.Before(*item.StartsAt) {
		return Coupon{}, 0, apiError{422, "COUPON_INVALID", "优惠码尚未生效"}
	}
	if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
		return Coupon{}, 0, apiError{422, "COUPON_INVALID", "优惠码已过期"}
	}
	if item.MaxUses > 0 && item.UsedCount >= item.MaxUses {
		return Coupon{}, 0, apiError{422, "COUPON_INVALID", "优惠码使用次数已用完"}
	}
	if priceCents < item.MinAmountCents {
		return Coupon{}, 0, apiError{422, "COUPON_INVALID", "订单金额未达到优惠门槛"}
	}
	discount := item.AmountCents
	if discount > priceCents {
		discount = priceCents
	}
	if discount <= 0 {
		return Coupon{}, 0, apiError{422, "COUPON_INVALID", "优惠金额不合法"}
	}
	return item, discount, nil
}

// ValidateCoupon 试算优惠券，不占用次数。
func (s *Service) ValidateCoupon(ctx context.Context, code, planID string) (CouponQuote, error) {
	var price int
	err := s.pool.QueryRow(ctx,
		`select price_cents from membership_plan where id = $1 and status = 'online'`, planID).Scan(&price)
	if errors.Is(err, pgx.ErrNoRows) {
		return CouponQuote{}, apiError{404, "PLAN_NOT_FOUND", "套餐不存在"}
	}
	if err != nil {
		return CouponQuote{}, err
	}
	item, discount, err := s.couponDiscount(ctx, s.pool, code, price, time.Now(), false)
	if err != nil {
		return CouponQuote{}, err
	}
	return CouponQuote{Code: item.Code, DiscountCents: discount, PayableCents: price - discount}, nil
}

// Reconciliation 按天汇总支付/退款/优惠。
func (s *Service) Reconciliation(ctx context.Context, from, to string) (ReconciliationReport, error) {
	end := time.Now()
	if to != "" {
		parsed, err := time.ParseInLocation("2006-01-02", to, time.Local)
		if err != nil {
			return ReconciliationReport{}, apiError{422, "VALIDATION_ERROR", "to 需为 YYYY-MM-DD"}
		}
		end = parsed
	}
	start := end.AddDate(0, 0, -29)
	if from != "" {
		parsed, err := time.ParseInLocation("2006-01-02", from, time.Local)
		if err != nil {
			return ReconciliationReport{}, apiError{422, "VALIDATION_ERROR", "from 需为 YYYY-MM-DD"}
		}
		start = parsed
	}
	if start.After(end) {
		return ReconciliationReport{}, apiError{422, "VALIDATION_ERROR", "from 不能晚于 to"}
	}
	if end.Sub(start) > 366*24*time.Hour {
		return ReconciliationReport{}, apiError{422, "VALIDATION_ERROR", "日期范围不能超过 366 天"}
	}

	rows, err := s.pool.Query(ctx, `
		select to_char(day, 'YYYY-MM-DD'),
		       sum(paid_count)::int, sum(paid_cents)::int,
		       sum(refunded_count)::int, sum(refunded_cents)::int,
		       sum(discount_cents)::int
		from (
			select paid_at::date as day, count(*)::int as paid_count,
			       coalesce(sum(amount_cents), 0)::int as paid_cents,
			       0 as refunded_count, 0 as refunded_cents,
			       coalesce(sum(discount_cents), 0)::int as discount_cents
			from membership_order
			where status in ('paid', 'refunded')
			  and paid_at >= $1::date and paid_at < ($2::date + 1)
			group by 1
			union all
			select refunded_at::date, 0, 0, count(*)::int,
			       coalesce(sum(amount_cents), 0)::int, 0
			from membership_order
			where status = 'refunded'
			  and refunded_at >= $1::date and refunded_at < ($2::date + 1)
			group by 1
		) t
		group by day
		order by day`,
		start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return ReconciliationReport{}, err
	}
	defer rows.Close()

	report := ReconciliationReport{
		From: start.Format("2006-01-02"),
		To:   end.Format("2006-01-02"),
		Days: make([]ReconciliationDay, 0),
	}
	for rows.Next() {
		var day ReconciliationDay
		if err := rows.Scan(&day.Date, &day.PaidCount, &day.PaidCents,
			&day.RefundedCount, &day.RefundedCents, &day.DiscountCents); err != nil {
			return ReconciliationReport{}, err
		}
		day.NetCents = day.PaidCents - day.RefundedCents
		report.Days = append(report.Days, day)
		report.Totals.PaidCount += day.PaidCount
		report.Totals.PaidCents += day.PaidCents
		report.Totals.RefundedCount += day.RefundedCount
		report.Totals.RefundedCents += day.RefundedCents
		report.Totals.DiscountCents += day.DiscountCents
	}
	report.Totals.NetCents = report.Totals.PaidCents - report.Totals.RefundedCents
	return report, rows.Err()
}

const invoiceColumns = `id, order_id, account_id, title_type, title, tax_no, email,
	amount_cents, status, remark, created_at, issued_at, updated_at`

func scanInvoice(row scanner) (Invoice, error) {
	var item Invoice
	err := row.Scan(&item.ID, &item.OrderID, &item.AccountID, &item.TitleType,
		&item.Title, &item.TaxNo, &item.Email, &item.AmountCents, &item.Status,
		&item.Remark, &item.CreatedAt, &item.IssuedAt, &item.UpdatedAt)
	return item, err
}

func validateInvoiceInput(in InvoiceInput) error {
	if in.TitleType == "" {
		in.TitleType = "personal"
	}
	if in.TitleType != "personal" && in.TitleType != "company" {
		return apiError{422, "VALIDATION_ERROR", "发票类型不合法"}
	}
	if strings.TrimSpace(in.Title) == "" || len([]rune(in.Title)) > 100 {
		return apiError{422, "VALIDATION_ERROR", "发票抬头需为 1–100 字"}
	}
	if in.TitleType == "company" && len(strings.TrimSpace(in.TaxNo)) < 6 {
		return apiError{422, "VALIDATION_ERROR", "企业抬头需填写税号"}
	}
	email := strings.TrimSpace(in.Email)
	if email != "" && (!strings.Contains(email, "@") || len(email) > 120) {
		return apiError{422, "VALIDATION_ERROR", "邮箱不合法"}
	}
	return nil
}

// CreateInvoice 为已支付订单提交发票申请。
func (s *Service) CreateInvoice(ctx context.Context, accountID, orderID string, in InvoiceInput) (Invoice, error) {
	if accountID == "" {
		return Invoice{}, apiError{401, "AUTH_REQUIRED", "家长未登录"}
	}
	if in.TitleType == "" {
		in.TitleType = "personal"
	}
	if err := validateInvoiceInput(in); err != nil {
		return Invoice{}, err
	}
	var status string
	var amount int
	err := s.pool.QueryRow(ctx, `
		select status, amount_cents from membership_order
		where id = $1 and account_id = $2`, orderID, accountID).Scan(&status, &amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, apiError{404, "ORDER_NOT_FOUND", "订单不存在"}
	}
	if err != nil {
		return Invoice{}, err
	}
	if status != "paid" {
		return Invoice{}, apiError{409, "ORDER_NOT_PAID", "只有已支付订单可以申请发票"}
	}
	item, err := scanInvoice(s.pool.QueryRow(ctx, `
		insert into membership_invoice (
			order_id, account_id, title_type, title, tax_no, email, amount_cents)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning `+invoiceColumns,
		orderID, accountID, in.TitleType, strings.TrimSpace(in.Title),
		strings.TrimSpace(in.TaxNo), strings.TrimSpace(in.Email), amount))
	if err != nil {
		if isUniqueViolation(err) {
			return Invoice{}, apiError{409, "INVOICE_EXISTS", "该订单已提交发票申请"}
		}
		return Invoice{}, err
	}
	return item, nil
}

// Invoices 返回当前账号的发票申请。
func (s *Service) Invoices(ctx context.Context, accountID string) ([]Invoice, error) {
	rows, err := s.pool.Query(ctx, `
		select `+invoiceColumns+` from membership_invoice
		where account_id = $1 order by created_at desc limit 50`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Invoice, 0)
	for rows.Next() {
		item, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// AdminInvoices 返回全部发票申请，可按状态筛选。
func (s *Service) AdminInvoices(ctx context.Context, status string, limit int) ([]Invoice, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		select `+invoiceColumns+` from membership_invoice
		where ($1 = '' or status = $1)
		order by created_at desc limit $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Invoice, 0)
	for rows.Next() {
		item, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// UpdateInvoiceStatus 由管理员标记已开具/驳回。
func (s *Service) UpdateInvoiceStatus(ctx context.Context, id, status, remark string) (Invoice, error) {
	if status != "issued" && status != "rejected" {
		return Invoice{}, apiError{422, "VALIDATION_ERROR", "状态需为 issued 或 rejected"}
	}
	if len([]rune(remark)) > 200 {
		return Invoice{}, apiError{422, "VALIDATION_ERROR", "备注最多 200 字"}
	}
	item, err := scanInvoice(s.pool.QueryRow(ctx, `
		update membership_invoice
		set status = $2, remark = $3, updated_at = now(),
		    issued_at = case when $2 = 'issued' then now() else issued_at end
		where id = $1
		returning `+invoiceColumns, id, status, strings.TrimSpace(remark)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, apiError{404, "INVOICE_NOT_FOUND", "发票申请不存在"}
	}
	if err != nil {
		return Invoice{}, err
	}
	return item, nil
}

func (s *Service) CreateOrder(ctx context.Context, accountID, planID string) (Order, error) {
	return s.CreateOrderWithCoupon(ctx, accountID, planID, "")
}

// CreateOrderWithCoupon 创建订单并可应用固定金额优惠券。
func (s *Service) CreateOrderWithCoupon(ctx context.Context, accountID, planID, couponCode string) (Order, error) {
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

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now()
	discount := 0
	appliedCode := ""
	if strings.TrimSpace(couponCode) != "" {
		var coupon Coupon
		coupon, discount, err = s.couponDiscount(ctx, tx, couponCode, plan.PriceCents, now, true)
		if err != nil {
			return Order{}, err
		}
		appliedCode = coupon.Code
		if _, err := tx.Exec(ctx, `
			update membership_coupon set used_count = used_count + 1, updated_at = now()
			where id = $1`, coupon.ID); err != nil {
			return Order{}, err
		}
	}
	amount := plan.PriceCents - discount

	item, err := scanOrder(tx.QueryRow(ctx, `
		insert into membership_order (
			order_no, account_id, plan_id, plan_name, amount_cents,
			original_amount_cents, discount_cents, coupon_code, duration_days)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		returning id, order_no, account_id, plan_id, plan_name, amount_cents,
		          original_amount_cents, discount_cents, coupon_code, duration_days,
		          status, channel, created_at, paid_at, refunded_at,
		          coalesce(refund_reason, ''), coalesce(refunded_by::text, ''),
		          coalesce(external_trade_no, ''),
		          coalesce(refund_before::text, ''),
		          coalesce(refund_after::text, '')`,
		orderNo(), accountID, plan.ID, plan.Name, amount,
		plan.PriceCents, discount, appliedCode, plan.DurationDays))
	if err != nil {
		return Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
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
		select id, order_no, account_id, plan_id, plan_name, amount_cents, original_amount_cents, discount_cents, coupon_code, duration_days,
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
		select id, order_no, account_id, plan_id, plan_name, amount_cents, original_amount_cents, discount_cents, coupon_code, duration_days,
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
		select id, order_no, account_id, plan_id, plan_name, amount_cents, original_amount_cents, discount_cents, coupon_code, duration_days,
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
	if item.CouponCode != "" {
		if _, err := tx.Exec(ctx, `
			update membership_coupon set used_count = greatest(used_count - 1, 0), updated_at = now()
			where code = $1`, item.CouponCode); err != nil {
			return Order{}, err
		}
	}
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
		&item.PlanName, &item.AmountCents, &item.OriginalAmountCents,
		&item.DiscountCents, &item.CouponCode, &item.DurationDays, &item.Status,
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
