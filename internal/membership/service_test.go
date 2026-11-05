package membership

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/roma007/OpenQuizGo/migrations"
)

func TestMembershipWorkflow(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	var accountID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'membership-test')
		returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	svc := New(pool, "test-callback-secret")

	// 种子套餐 + 测试套餐。
	seeded, err := svc.ListPlans(ctx, true)
	if err != nil {
		t.Fatalf("list seeded: %v", err)
	}
	if len(seeded) < 2 {
		t.Fatalf("seeded plans = %d, want >= 2", len(seeded))
	}

	plan, err := svc.CreatePlan(ctx, PlanInput{
		Code: "test-daily", Name: "测试日卡", Description: "测试",
		DurationDays: 1, PriceCents: 100, Features: []string{"权益 A"},
		EntitlementCodes: []string{"topic_pack"}, Status: "online", Sort: 1,
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if plan.Code != "test-daily" || plan.PriceCents != 100 || len(plan.Features) != 1 ||
		len(plan.EntitlementCodes) != 1 || plan.EntitlementCodes[0] != "topic_pack" {
		t.Fatalf("unexpected plan: %+v", plan)
	}

	if _, err := svc.CreatePlan(ctx, PlanInput{
		Code: "test-daily", Name: "重复", DurationDays: 1, Status: "online",
		EntitlementCodes: []string{"topic_pack"},
	}); err == nil || !isCode(err, "PLAN_CODE_EXISTS") {
		t.Fatalf("duplicate code = %v, want PLAN_CODE_EXISTS", err)
	}
	if _, err := svc.CreatePlan(ctx, PlanInput{
		Code: "Bad Code", Name: "x", DurationDays: 1, Status: "online",
	}); err == nil || !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("bad code = %v, want VALIDATION_ERROR", err)
	}
	if _, err := svc.CreatePlan(ctx, PlanInput{
		Code: "test-bad-ent", Name: "x", DurationDays: 1, Status: "online",
		EntitlementCodes: []string{"unknown"},
	}); err == nil || !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("bad entitlement = %v, want VALIDATION_ERROR", err)
	}

	updated, err := svc.UpdatePlan(ctx, plan.ID, PlanInput{
		Code: "test-daily", Name: "测试日卡（改）", Description: "改",
		DurationDays: 1, PriceCents: 150, Features: []string{"权益 A", "权益 B"},
		EntitlementCodes: []string{"topic_pack", "report_advanced"},
		Status:           "online", Sort: 2,
	})
	if err != nil {
		t.Fatalf("update plan: %v", err)
	}
	if updated.Name != "测试日卡（改）" || updated.PriceCents != 150 || len(updated.Features) != 2 ||
		len(updated.EntitlementCodes) != 2 {
		t.Fatalf("unexpected updated plan: %+v", updated)
	}

	// 下单与支付。
	order, err := svc.CreateOrder(ctx, accountID, plan.ID)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.Status != "pending" || order.AmountCents != 150 || order.DurationDays != 1 {
		t.Fatalf("unexpected order: %+v", order)
	}
	paid, err := svc.PayOrder(ctx, accountID, order.ID)
	if err != nil {
		t.Fatalf("pay order: %v", err)
	}
	if paid.Order.Status != "paid" || !paid.Membership.Active || paid.Membership.Status != "active" ||
		len(paid.Membership.Entitlements) != 2 {
		t.Fatalf("unexpected pay result: %+v", paid)
	}
	firstExpiry := *paid.Membership.ExpiresAt

	// 重复支付幂等，不重复累加。
	again, err := svc.PayOrder(ctx, accountID, order.ID)
	if err != nil {
		t.Fatalf("pay again: %v", err)
	}
	if !again.Membership.ExpiresAt.Equal(firstExpiry) {
		t.Fatalf("idempotent pay changed expiry: %v -> %v", firstExpiry, again.Membership.ExpiresAt)
	}

	// 新订单续费从当前到期时间累加。
	order2, err := svc.CreateOrder(ctx, accountID, plan.ID)
	if err != nil {
		t.Fatalf("create order2: %v", err)
	}
	renewed, err := svc.PayOrder(ctx, accountID, order2.ID)
	if err != nil {
		t.Fatalf("pay order2: %v", err)
	}
	if diff := renewed.Membership.ExpiresAt.Sub(firstExpiry); diff != 24*time.Hour {
		t.Fatalf("renewal diff = %v, want 24h", diff)
	}

	center, err := svc.Center(ctx, accountID)
	if err != nil {
		t.Fatalf("center: %v", err)
	}
	if center.Membership == nil || !center.Membership.Active || len(center.Plans) < 3 {
		t.Fatalf("unexpected center: %+v", center)
	}
	orders, err := svc.Orders(ctx, accountID)
	if err != nil {
		t.Fatalf("orders: %v", err)
	}
	if len(orders) != 2 || orders[0].Status != "paid" {
		t.Fatalf("unexpected orders: %+v", orders)
	}

	// 退款：回退一单时长，重复退款幂等。
	refunded, err := svc.RefundOrder(ctx, order2.ID, "用户申请退款", "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("refund order2: %v", err)
	}
	if refunded.Status != "refunded" || refunded.RefundReason != "用户申请退款" ||
		refunded.RefundedBy != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("unexpected refund: %+v", refunded)
	}
	before := decodeRefundSnapshot(t, refunded.RefundBefore)
	after := decodeRefundSnapshot(t, refunded.RefundAfter)
	if before.Status != "active" || before.PlanName != updated.Name ||
		!before.ExpiresAt.Equal(*renewed.Membership.ExpiresAt) {
		t.Fatalf("unexpected refund before snapshot: %+v", before)
	}
	if after.Status != "active" || after.PlanName != updated.Name ||
		!after.ExpiresAt.Equal(firstExpiry) {
		t.Fatalf("unexpected refund after snapshot: %+v", after)
	}

	// 审计记录携带退款前后快照。
	svc.audit(ctx, "00000000-0000-0000-0000-000000000001", refunded, "用户申请退款", "")
	var auditData string
	if err := pool.QueryRow(ctx, `
		select after_data::text from audit_log
		where target_id = $1 and action = 'membership.order.refund'
		order by id desc limit 1`, order2.ID).Scan(&auditData); err != nil {
		t.Fatalf("audit after_data: %v", err)
	}
	var auditPayload struct {
		OrderNo string         `json:"order_no"`
		Reason  string         `json:"reason"`
		Before  refundSnapshot `json:"before"`
		After   refundSnapshot `json:"after"`
	}
	if err := json.Unmarshal([]byte(auditData), &auditPayload); err != nil {
		t.Fatalf("decode audit after_data: %v", err)
	}
	if auditPayload.OrderNo != order2.OrderNo || auditPayload.Reason != "用户申请退款" ||
		auditPayload.Before.Status != "active" || auditPayload.After.Status != "active" ||
		!auditPayload.Before.ExpiresAt.Equal(before.ExpiresAt) ||
		!auditPayload.After.ExpiresAt.Equal(after.ExpiresAt) {
		t.Fatalf("unexpected audit after_data: %+v", auditPayload)
	}

	afterRefund, err := svc.Center(ctx, accountID)
	if err != nil {
		t.Fatalf("center after refund: %v", err)
	}
	if afterRefund.Membership == nil || !afterRefund.Membership.ExpiresAt.Equal(firstExpiry) {
		t.Fatalf("refund did not roll back expiry: %+v", afterRefund.Membership)
	}
	againRefund, err := svc.RefundOrder(ctx, order2.ID, "再次退款", "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("refund twice: %v", err)
	}
	afterTwice, _ := svc.Center(ctx, accountID)
	if !againRefund.RefundedAt.Equal(*refunded.RefundedAt) ||
		!afterTwice.Membership.ExpiresAt.Equal(firstExpiry) ||
		againRefund.RefundReason != "用户申请退款" {
		t.Fatal("idempotent refund changed expiry or reason")
	}
	if canonicalJSON(t, againRefund.RefundBefore) != canonicalJSON(t, refunded.RefundBefore) ||
		canonicalJSON(t, againRefund.RefundAfter) != canonicalJSON(t, refunded.RefundAfter) {
		t.Fatal("idempotent refund changed snapshots")
	}

	adminOrders, total, err := svc.AdminOrders(ctx, "refunded", 20, 0)
	if err != nil {
		t.Fatalf("admin orders: %v", err)
	}
	if total != 1 || len(adminOrders) != 1 || adminOrders[0].ID != order2.ID {
		t.Fatalf("unexpected admin orders: total=%d items=%+v", total, adminOrders)
	}
	if len(adminOrders[0].RefundBefore) == 0 || len(adminOrders[0].RefundAfter) == 0 {
		t.Fatalf("admin orders missing refund snapshots: %+v", adminOrders[0])
	}

	// 未支付订单不能退款。
	pending, err := svc.CreateOrder(ctx, accountID, plan.ID)
	if err != nil {
		t.Fatalf("pending order: %v", err)
	}
	if _, err := svc.RefundOrder(ctx, pending.ID, "", "00000000-0000-0000-0000-000000000001"); err == nil || !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("empty reason = %v, want VALIDATION_ERROR", err)
	}
	if _, err := svc.RefundOrder(ctx, pending.ID, "测试", "00000000-0000-0000-0000-000000000001"); err == nil || !isCode(err, "ORDER_NOT_REFUNDABLE") {
		t.Fatalf("refund pending = %v, want ORDER_NOT_REFUNDABLE", err)
	}

	// 退掉首单后会员到期，权益失效。
	fullRefund, err := svc.RefundOrder(ctx, order.ID, "全额退款", "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("refund order1: %v", err)
	}
	fullBefore := decodeRefundSnapshot(t, fullRefund.RefundBefore)
	fullAfter := decodeRefundSnapshot(t, fullRefund.RefundAfter)
	if fullBefore.Status != "active" || !fullBefore.ExpiresAt.Equal(firstExpiry) {
		t.Fatalf("unexpected full refund before snapshot: %+v", fullBefore)
	}
	if fullAfter.Status != "expired" || fullAfter.ExpiresAt.After(time.Now()) {
		t.Fatalf("unexpected full refund after snapshot: %+v", fullAfter)
	}
	expiredCenter, err := svc.Center(ctx, accountID)
	if err != nil {
		t.Fatalf("center after full refund: %v", err)
	}
	if expiredCenter.Membership == nil || expiredCenter.Membership.Active ||
		len(expiredCenter.Membership.Entitlements) != 0 {
		t.Fatalf("membership still active after refund: %+v", expiredCenter.Membership)
	}

	// 有订单引用时删除套餐失败。
	if err := svc.DeletePlan(ctx, plan.ID); err == nil || !isCode(err, "PLAN_IN_USE") {
		t.Fatalf("delete used plan = %v, want PLAN_IN_USE", err)
	}

	// 未上线套餐不能下单。
	offline, err := svc.CreatePlan(ctx, PlanInput{
		Code: "test-offline", Name: "下线卡", DurationDays: 7, PriceCents: 700,
		Status: "offline", Sort: 3,
	})
	if err != nil {
		t.Fatalf("create offline: %v", err)
	}
	if _, err := svc.CreateOrder(ctx, accountID, offline.ID); err == nil || !isCode(err, "PLAN_OFFLINE") {
		t.Fatalf("offline order = %v, want PLAN_OFFLINE", err)
	}
	if err := svc.DeletePlan(ctx, offline.ID); err != nil {
		t.Fatalf("delete offline plan: %v", err)
	}

	// 到期后状态为 expired。
	if _, err := pool.Exec(ctx, `
		update account_membership set expires_at = now() - interval '1 hour' where account_id = $1`,
		accountID); err != nil {
		t.Fatalf("expire: %v", err)
	}
	expired, err := svc.Center(ctx, accountID)
	if err != nil {
		t.Fatalf("center expired: %v", err)
	}
	if expired.Membership == nil || expired.Membership.Active ||
		expired.Membership.Status != "expired" || len(expired.Membership.Entitlements) != 0 {
		t.Fatalf("unexpected expired membership: %+v", expired.Membership)
	}
}

func isCode(err error, code string) bool {
	var e apiError
	return errors.As(err, &e) && e.Code == code
}

type refundSnapshot struct {
	Status    string    `json:"status"`
	PlanName  string    `json:"plan_name"`
	ExpiresAt time.Time `json:"expires_at"`
}

func decodeRefundSnapshot(t *testing.T, raw json.RawMessage) refundSnapshot {
	t.Helper()
	if len(raw) == 0 {
		t.Fatal("missing refund snapshot")
	}
	var snapshot refundSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode refund snapshot: %v", err)
	}
	return snapshot
}

func TestFamilyMembershipSharing(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	var ownerID, memberID, familyID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('c', 'share-owner')
		returning id`).Scan(&ownerID); err != nil {
		t.Fatalf("owner: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('c', 'share-member')
		returning id`).Scan(&memberID); err != nil {
		t.Fatalf("member: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		insert into family (owner_id) values ($1) returning id`, ownerID).Scan(&familyID); err != nil {
		t.Fatalf("family: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into family_member (family_id, account_id, role)
		values ($1, $2, 'member')`, familyID, memberID); err != nil {
		t.Fatalf("member row: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`update parent_account set active_family_id = $2 where id = any($1::uuid[])`,
		[]string{ownerID, memberID}, familyID); err != nil {
		t.Fatalf("active family: %v", err)
	}

	var planID string
	if err := pool.QueryRow(ctx, `
		insert into membership_plan (code, name, duration_days, price_cents, features, entitlement_codes, status, sort)
		values ('share-plan', '共享卡', 30, 0, '[]'::jsonb, '["topic_pack"]'::jsonb, 'online', 1)
		returning id`).Scan(&planID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into account_membership (account_id, plan_id, plan_name, status, starts_at, expires_at)
		values ($1, $2, '共享卡', 'active', now(), now() + interval '30 days')`,
		ownerID, planID); err != nil {
		t.Fatalf("membership: %v", err)
	}

	svc := New(pool, "")
	center, err := svc.Center(ctx, memberID)
	if err != nil {
		t.Fatalf("center member: %v", err)
	}
	if center.Membership == nil || !center.Membership.Active || !center.Membership.Shared {
		t.Fatalf("expected shared membership: %+v", center.Membership)
	}
	setting, err := svc.Sharing(ctx, memberID)
	if err != nil || !setting.Shared || setting.CanEdit {
		t.Fatalf("unexpected member sharing: %+v err=%v", setting, err)
	}
	if _, err := svc.SetSharing(ctx, memberID, false); !isCode(err, "FORBIDDEN") {
		t.Fatalf("member set sharing = %v, want FORBIDDEN", err)
	}
	if _, err := svc.SetSharing(ctx, ownerID, false); err != nil {
		t.Fatalf("owner disable: %v", err)
	}
	center, err = svc.Center(ctx, memberID)
	if err != nil {
		t.Fatalf("center after disable: %v", err)
	}
	if center.Membership != nil && center.Membership.Active {
		t.Fatalf("shared membership should be hidden: %+v", center.Membership)
	}
	if _, err := svc.SetSharing(ctx, ownerID, true); err != nil {
		t.Fatalf("owner enable: %v", err)
	}
	center, err = svc.Center(ctx, memberID)
	if err != nil || center.Membership == nil || !center.Membership.Shared {
		t.Fatalf("shared membership not restored: %+v err=%v", center.Membership, err)
	}
}

func TestCouponWorkflow(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	var accountID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'coupon-test')
		returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	svc := New(pool, "")
	plan, err := svc.CreatePlan(ctx, PlanInput{
		Code: "coupon-plan", Name: "优惠券套餐", DurationDays: 30, PriceCents: 1000,
		Status: "online", EntitlementCodes: []string{"topic_pack"},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	coupon, err := svc.CreateCoupon(ctx, CouponInput{
		Code: "save100", Name: "立减 1 元", AmountCents: 100,
		MinAmountCents: 500, MaxUses: 2, Status: "active",
	})
	if err != nil {
		t.Fatalf("coupon: %v", err)
	}
	if coupon.Code != "SAVE100" {
		t.Fatalf("coupon code = %s", coupon.Code)
	}

	quote, err := svc.ValidateCoupon(ctx, "save100", plan.ID)
	if err != nil || quote.DiscountCents != 100 || quote.PayableCents != 900 {
		t.Fatalf("quote = %+v err=%v", quote, err)
	}

	order, err := svc.CreateOrderWithCoupon(ctx, accountID, plan.ID, "save100")
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if order.AmountCents != 900 || order.OriginalAmountCents != 1000 ||
		order.DiscountCents != 100 || order.CouponCode != "SAVE100" {
		t.Fatalf("unexpected order: %+v", order)
	}
	var used int
	if err := pool.QueryRow(ctx,
		`select used_count from membership_coupon where id = $1`, coupon.ID).Scan(&used); err != nil {
		t.Fatalf("used: %v", err)
	}
	if used != 1 {
		t.Fatalf("used = %d, want 1", used)
	}

	if _, err := svc.CreateOrderWithCoupon(ctx, accountID, plan.ID, "bad-code"); err == nil {
		t.Fatal("expected invalid coupon error")
	}

	if _, err := svc.PayOrder(ctx, accountID, order.ID); err != nil {
		t.Fatalf("pay: %v", err)
	}
	if _, err := svc.RefundOrder(ctx, order.ID, "测试退款", ""); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`select used_count from membership_coupon where id = $1`, coupon.ID).Scan(&used); err != nil {
		t.Fatalf("used after refund: %v", err)
	}
	if used != 0 {
		t.Fatalf("used after refund = %d, want 0", used)
	}
}

func TestInvoiceWorkflow(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	var accountID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'invoice-test')
		returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	svc := New(pool, "")
	plan, err := svc.CreatePlan(ctx, PlanInput{
		Code: "invoice-plan", Name: "发票套餐", DurationDays: 30, PriceCents: 1000,
		Status: "online", EntitlementCodes: []string{"topic_pack"},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	order, err := svc.CreateOrder(ctx, accountID, plan.ID)
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if _, err := svc.CreateInvoice(ctx, accountID, order.ID, InvoiceInput{
		TitleType: "personal", Title: "张三",
	}); err == nil {
		t.Fatal("expected unpaid order rejection")
	}
	if _, err := svc.PayOrder(ctx, accountID, order.ID); err != nil {
		t.Fatalf("pay: %v", err)
	}
	invoice, err := svc.CreateInvoice(ctx, accountID, order.ID, InvoiceInput{
		TitleType: "personal", Title: "张三", Email: "a@example.com",
	})
	if err != nil {
		t.Fatalf("invoice: %v", err)
	}
	if invoice.AmountCents != 1000 || invoice.Status != "requested" || invoice.OrderID != order.ID {
		t.Fatalf("unexpected invoice: %+v", invoice)
	}
	if _, err := svc.CreateInvoice(ctx, accountID, order.ID, InvoiceInput{
		Title: "张三",
	}); !isCode(err, "INVOICE_EXISTS") {
		t.Fatalf("duplicate = %v, want INVOICE_EXISTS", err)
	}
	if _, err := svc.UpdateInvoiceStatus(ctx, invoice.ID, "issued", "已开具"); err != nil {
		t.Fatalf("issue: %v", err)
	}
	items, err := svc.Invoices(ctx, accountID)
	if err != nil || len(items) != 1 || items[0].Status != "issued" {
		t.Fatalf("invoices = %+v err=%v", items, err)
	}
	adminItems, err := svc.AdminInvoices(ctx, "issued", 10)
	if err != nil || len(adminItems) != 1 {
		t.Fatalf("admin invoices = %+v err=%v", adminItems, err)
	}
	if _, err := svc.CreateInvoice(ctx, accountID, order.ID, InvoiceInput{
		TitleType: "company", Title: "公司",
	}); err == nil {
		t.Fatal("expected duplicate before tax validation")
	}
}

func TestReconciliationReport(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	var accountID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'reconcile-test')
		returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	svc := New(pool, "")
	plan, err := svc.CreatePlan(ctx, PlanInput{
		Code: "reconcile-plan", Name: "对账套餐", DurationDays: 30, PriceCents: 1000,
		Status: "online", EntitlementCodes: []string{"topic_pack"},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	order1, err := svc.CreateOrder(ctx, accountID, plan.ID)
	if err != nil {
		t.Fatalf("order1: %v", err)
	}
	if _, err := svc.PayOrder(ctx, accountID, order1.ID); err != nil {
		t.Fatalf("pay1: %v", err)
	}
	order2, err := svc.CreateOrder(ctx, accountID, plan.ID)
	if err != nil {
		t.Fatalf("order2: %v", err)
	}
	if _, err := svc.PayOrder(ctx, accountID, order2.ID); err != nil {
		t.Fatalf("pay2: %v", err)
	}
	if _, err := svc.RefundOrder(ctx, order2.ID, "对账测试", ""); err != nil {
		t.Fatalf("refund: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	report, err := svc.Reconciliation(ctx, today, today)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(report.Days) != 1 {
		t.Fatalf("days = %+v", report.Days)
	}
	day := report.Days[0]
	if day.PaidCount != 2 || day.PaidCents != 2000 ||
		day.RefundedCount != 1 || day.RefundedCents != 1000 || day.NetCents != 1000 {
		t.Fatalf("unexpected day: %+v", day)
	}
	if report.Totals.NetCents != 1000 {
		t.Fatalf("totals = %+v", report.Totals)
	}
	if _, err := svc.Reconciliation(ctx, "2026-11-10", "2026-11-01"); !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("invalid range = %v", err)
	}
}

func TestRefundCallback(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	var accountID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'refund-cb-test')
		returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	svc := New(pool, "pay-secret").WithRefundSecret("refund-secret")
	plan, err := svc.CreatePlan(ctx, PlanInput{
		Code: "refund-cb-plan", Name: "退款回调套餐", DurationDays: 30, PriceCents: 1000,
		Status: "online", EntitlementCodes: []string{"topic_pack"},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	order, err := svc.CreateOrder(ctx, accountID, plan.ID)
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if _, err := svc.PayOrder(ctx, accountID, order.ID); err != nil {
		t.Fatalf("pay: %v", err)
	}
	var expiryBefore time.Time
	if err := pool.QueryRow(ctx,
		`select expires_at from account_membership where account_id = $1`, accountID).Scan(&expiryBefore); err != nil {
		t.Fatalf("expiry before: %v", err)
	}

	body := []byte(fmt.Sprintf(
		`{"order_no":%q,"refund_no":"R-1","status":"refunded","reason":"用户申请"}`,
		order.OrderNo))
	mac := hmac.New(sha256.New, []byte("refund-secret"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	refunded, err := svc.RefundCallback(ctx, body, sig)
	if err != nil {
		t.Fatalf("refund callback: %v", err)
	}
	if refunded.Status != "refunded" || refunded.ExternalRefundNo != "R-1" {
		t.Fatalf("unexpected refunded order: %+v", refunded)
	}
	var expiryAfter time.Time
	if err := pool.QueryRow(ctx,
		`select expires_at from account_membership where account_id = $1`, accountID).Scan(&expiryAfter); err != nil {
		t.Fatalf("expiry after: %v", err)
	}
	if !expiryAfter.Before(expiryBefore) {
		t.Fatalf("expiry not reduced: %v -> %v", expiryBefore, expiryAfter)
	}

	// 重复回调幂等，不再次回退。
	again, err := svc.RefundCallback(ctx, body, sig)
	if err != nil {
		t.Fatalf("second refund callback: %v", err)
	}
	if again.Status != "refunded" {
		t.Fatalf("second status = %s", again.Status)
	}
	var expiryAgain time.Time
	if err := pool.QueryRow(ctx,
		`select expires_at from account_membership where account_id = $1`, accountID).Scan(&expiryAgain); err != nil {
		t.Fatalf("expiry again: %v", err)
	}
	if !expiryAgain.Equal(expiryAfter) {
		t.Fatalf("double refund: %v -> %v", expiryAfter, expiryAgain)
	}

	if _, err := svc.RefundCallback(ctx, body, "bad"); !isCode(err, "SIGNATURE_INVALID") {
		t.Fatalf("bad signature = %v", err)
	}
	disabled := New(pool, "")
	if _, err := disabled.RefundCallback(ctx, body, sig); !isCode(err, "REFUND_DISABLED") {
		t.Fatalf("disabled = %v", err)
	}
}

func prepareDB(t *testing.T, url string) {
	t.Helper()
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`truncate table
		membership_invoice, account_membership, membership_order, membership_coupon,
		membership_plan, parent_account cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	// 重新插入种子套餐，避免被 truncate 清空。
	if _, err := db.Exec(`insert into membership_plan (code, name, description, duration_days, price_cents, features, status, sort)
		values ('monthly', '月度会员', '', 30, 1990, '[]'::jsonb, 'online', 10),
		       ('yearly', '年度会员', '', 365, 16900, '[]'::jsonb, 'online', 20)`); err != nil {
		t.Fatalf("seed plans: %v", err)
	}
}

func TestPaymentCallback(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	var accountID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'callback-test')
		returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	svc := New(pool, "test-callback-secret")
	plan, err := svc.CreatePlan(ctx, PlanInput{
		Code: "callback-plan", Name: "回调测试卡", DurationDays: 7, PriceCents: 100,
		Features: []string{}, EntitlementCodes: []string{}, Status: "online",
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	order, err := svc.CreateOrder(ctx, accountID, plan.ID)
	if err != nil {
		t.Fatalf("order: %v", err)
	}

	body := []byte(`{"order_no":"` + order.OrderNo + `","trade_no":"T123","status":"paid"}`)
	mac := hmac.New(sha256.New, []byte("test-callback-secret"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if _, err := svc.Callback(ctx, body, "bad-signature"); err == nil || !isCode(err, "SIGNATURE_INVALID") {
		t.Fatalf("bad signature = %v, want SIGNATURE_INVALID", err)
	}
	paid, err := svc.Callback(ctx, body, sig)
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if paid.Status != "paid" || paid.Channel != "callback" || paid.ExternalTradeNo != "T123" {
		t.Fatalf("unexpected paid order: %+v", paid)
	}
	center, err := svc.Center(ctx, accountID)
	if err != nil {
		t.Fatalf("center: %v", err)
	}
	if center.Membership == nil || !center.Membership.Active {
		t.Fatalf("membership not active: %+v", center.Membership)
	}
	expiry := *center.Membership.ExpiresAt

	// 重复回调幂等。
	if _, err := svc.Callback(ctx, body, sig); err != nil {
		t.Fatalf("callback again: %v", err)
	}
	center, _ = svc.Center(ctx, accountID)
	if !center.Membership.ExpiresAt.Equal(expiry) {
		t.Fatal("idempotent callback changed expiry")
	}

	// 未知订单。
	unknown := []byte(`{"order_no":"M-UNKNOWN","trade_no":"T1","status":"paid"}`)
	mac = hmac.New(sha256.New, []byte("test-callback-secret"))
	mac.Write(unknown)
	if _, err := svc.Callback(ctx, unknown, hex.EncodeToString(mac.Sum(nil))); err == nil || !isCode(err, "ORDER_NOT_FOUND") {
		t.Fatalf("unknown order = %v, want ORDER_NOT_FOUND", err)
	}

	// 未配置密钥时禁用回调。
	disabled := New(pool, "")
	if _, err := disabled.Callback(ctx, body, sig); err == nil || !isCode(err, "PAYMENT_DISABLED") {
		t.Fatalf("disabled callback = %v, want PAYMENT_DISABLED", err)
	}
}

func canonicalJSON(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	if len(raw) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode json: %v", err)
	}
	return string(encoded)
}
