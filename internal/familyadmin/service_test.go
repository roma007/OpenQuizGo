package familyadmin_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/roma007/OpenQuizGo/internal/familyadmin"
	"github.com/roma007/OpenQuizGo/migrations"
)

func TestFamilyListAndDetail(t *testing.T) {
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

	var accountID, familyID, childID, planID string
	if err := pool.QueryRow(ctx, `insert into parent_account (phone_cipher, phone_hash)
		values ('cipher','familyadmin-test') returning id`).Scan(&accountID); err != nil {
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
		values ($1,'查询孩子','low') returning id`, familyID).Scan(&childID); err != nil {
		t.Fatalf("child: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into device_bind (device_id, child_id, platform)
		values ('tv-query-test',$1,'tv')`, childID); err != nil {
		t.Fatalf("device: %v", err)
	}
	if err := pool.QueryRow(ctx, `insert into membership_plan (code,name,duration_days,price_cents,status)
		values ('familyadmin-plan','查询套餐',30,0,'online') returning id`).Scan(&planID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into account_membership
		(account_id,plan_id,plan_name,status,starts_at,expires_at)
		values ($1,$2,'查询套餐','active',now(),now() + interval '30 days')`,
		accountID, planID); err != nil {
		t.Fatalf("membership: %v", err)
	}

	svc := familyadmin.New(pool)
	list, err := svc.List(ctx, "查询孩子", 20, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("unexpected list: %+v", list)
	}
	item := list.Items[0]
	if item.FamilyID != familyID || item.ChildrenCount != 1 || item.DevicesCount != 1 ||
		item.MembershipStatus != "active" || item.PlanName != "查询套餐" {
		t.Fatalf("unexpected item: %+v", item)
	}

	byDevice, err := svc.List(ctx, "tv-query-test", 20, 0)
	if err != nil || byDevice.Total != 1 {
		t.Fatalf("device search: %+v err=%v", byDevice, err)
	}

	detail, err := svc.Get(ctx, familyID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if len(detail.MemberList) != 1 || len(detail.ChildList) != 1 || len(detail.DeviceList) != 1 {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if detail.DeviceList[0].DeviceID != "tv-query-test" || detail.DeviceList[0].ChildName != "查询孩子" {
		t.Fatalf("unexpected device: %+v", detail.DeviceList[0])
	}

	if _, err := svc.Get(ctx, "00000000-0000-0000-0000-000000000000"); err == nil {
		t.Fatal("expected not found")
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
		device_bind, child_profile, family_member, family, parent_account,
		account_membership, membership_plan cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}
