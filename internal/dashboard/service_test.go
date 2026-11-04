package dashboard_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/roma007/OpenQuizGo/internal/dashboard"
	"github.com/roma007/OpenQuizGo/migrations"
)

func TestOverview(t *testing.T) {
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

	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}

	for _, status := range []string{"online", "online", "draft"} {
		mustExec(`insert into question (subject_code, category_code, stage_code, difficulty,
			type_code, stem, options, answer, status)
			values ('math','core','low',2,'choice','概览题','[{"id":"a","text":"1"}]'::jsonb,'["a"]'::jsonb,$1)`, status)
	}
	mustExec(`insert into topic_pack (name,type,subject_scope,stage_scope,status)
		values ('在线专题','vacation','[]'::jsonb,'[]'::jsonb,'online'),
		       ('草稿专题','vacation','[]'::jsonb,'[]'::jsonb,'draft')`)

	var accountID, familyID, childID, planID string
	if err := pool.QueryRow(ctx, `insert into parent_account (phone_cipher, phone_hash)
		values ('cipher','dashboard-test') returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	if err := pool.QueryRow(ctx, `insert into family (owner_id) values ($1) returning id`,
		accountID).Scan(&familyID); err != nil {
		t.Fatalf("family: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into family_member (family_id, account_id, role, second_factor_verified)
		values ($1,$2,'owner',true)`, familyID, accountID); err != nil {
		t.Fatalf("member: %v", err)
	}
	if err := pool.QueryRow(ctx, `insert into child_profile (family_id, nickname, stage_code)
		values ($1,'概览孩子','low') returning id`, familyID).Scan(&childID); err != nil {
		t.Fatalf("child: %v", err)
	}
	if err := pool.QueryRow(ctx, `insert into membership_plan (code,name,duration_days,price_cents,status)
		values ('dashboard-plan','概览套餐',30,1990,'online') returning id`).Scan(&planID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into membership_order
		(order_no,account_id,plan_id,plan_name,amount_cents,duration_days,status,paid_at)
		values ('M-DASH-1',$1,$2,'概览套餐',1990,30,'paid',now()),
		       ('M-DASH-2',$1,$2,'概览套餐',990,7,'refunded',now() - interval '1 day')`,
		accountID, planID); err != nil {
		t.Fatalf("orders: %v", err)
	}
	mustExec(`insert into account_membership
		(account_id,plan_id,plan_name,status,starts_at,expires_at)
		values ($1,$2,'概览套餐','active',now(),now() + interval '3 days')`,
		accountID, planID)
	mustExec(`insert into family_battle (family_id,child_id,status,question_ids,finished_at)
		values ($1,$2,'finished','[]'::jsonb,now())`, familyID, childID)

	svc := dashboard.New(pool)
	out, err := svc.Overview(ctx)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if out.Questions.Total != 3 || out.Questions.Online != 2 || out.Questions.Draft != 1 {
		t.Fatalf("unexpected questions: %+v", out.Questions)
	}
	if out.Topics.Total != 2 || out.Topics.Online != 1 || out.Topics.Active != 1 {
		t.Fatalf("unexpected topics: %+v", out.Topics)
	}
	if out.Membership.ActiveMembers != 1 || out.Membership.RevenueCents != 1000 ||
		out.Membership.Orders30d != 2 || out.Membership.Paid30d != 1 {
		t.Fatalf("unexpected membership: %+v", out.Membership)
	}
	if out.Families != 1 || out.Children != 1 || out.Battles7d != 1 {
		t.Fatalf("unexpected counts: families=%d children=%d battles=%d",
			out.Families, out.Children, out.Battles7d)
	}
	if len(out.RecentOrders) != 2 {
		t.Fatalf("recent orders = %d, want 2", len(out.RecentOrders))
	}
	if len(out.ExpiringMemberships) != 1 || out.ExpiringMemberships[0].DaysRemaining < 1 {
		t.Fatalf("unexpected expiring: %+v", out.ExpiringMemberships)
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
		family_battle, family_battle_answer, membership_order, account_membership, membership_plan,
		topic_question, topic_pack, question,
		child_profile, family_member, family, parent_account
		cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}
