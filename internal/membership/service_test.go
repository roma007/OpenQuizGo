package membership

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
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

	adminOrders, total, err := svc.AdminOrders(ctx, "refunded", 20, 0)
	if err != nil {
		t.Fatalf("admin orders: %v", err)
	}
	if total != 1 || len(adminOrders) != 1 || adminOrders[0].ID != order2.ID {
		t.Fatalf("unexpected admin orders: total=%d items=%+v", total, adminOrders)
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
	if _, err := svc.RefundOrder(ctx, order.ID, "全额退款", "00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("refund order1: %v", err)
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
		account_membership, membership_order, membership_plan,
		parent_account cascade`); err != nil {
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
