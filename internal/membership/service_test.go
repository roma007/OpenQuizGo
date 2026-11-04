package membership

import (
	"context"
	"database/sql"
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
	svc := New(pool)

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
		Status: "online", Sort: 1,
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if plan.Code != "test-daily" || plan.PriceCents != 100 || len(plan.Features) != 1 {
		t.Fatalf("unexpected plan: %+v", plan)
	}

	if _, err := svc.CreatePlan(ctx, PlanInput{
		Code: "test-daily", Name: "重复", DurationDays: 1, Status: "online",
	}); err == nil || !isCode(err, "PLAN_CODE_EXISTS") {
		t.Fatalf("duplicate code = %v, want PLAN_CODE_EXISTS", err)
	}
	if _, err := svc.CreatePlan(ctx, PlanInput{
		Code: "Bad Code", Name: "x", DurationDays: 1, Status: "online",
	}); err == nil || !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("bad code = %v, want VALIDATION_ERROR", err)
	}

	updated, err := svc.UpdatePlan(ctx, plan.ID, PlanInput{
		Code: "test-daily", Name: "测试日卡（改）", Description: "改",
		DurationDays: 1, PriceCents: 150, Features: []string{"权益 A", "权益 B"},
		Status: "online", Sort: 2,
	})
	if err != nil {
		t.Fatalf("update plan: %v", err)
	}
	if updated.Name != "测试日卡（改）" || updated.PriceCents != 150 || len(updated.Features) != 2 {
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
	if paid.Order.Status != "paid" || !paid.Membership.Active || paid.Membership.Status != "active" {
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
	if expired.Membership == nil || expired.Membership.Active || expired.Membership.Status != "expired" {
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
