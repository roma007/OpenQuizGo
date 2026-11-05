package experiment

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/roma007/OpenQuizGo/migrations"
)

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
	if _, err := db.Exec(`truncate table experiment_exposure, experiment, parent_account cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestExperimentAssignment(t *testing.T) {
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
		insert into parent_account (phone_cipher, phone_hash) values ('c','experiment-test')
		returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	svc := New(pool)
	created, err := svc.Create(ctx, Input{
		Key: "home-cta", Name: "首页按钮", VariantA: "a", VariantB: "b",
		TrafficPercent: 50, Status: "active",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Key != "home-cta" {
		t.Fatalf("unexpected experiment: %+v", created)
	}
	if _, err := svc.Create(ctx, Input{
		Key: "home-cta", Name: "重复", VariantA: "a", VariantB: "b",
		TrafficPercent: 50, Status: "active",
	}); err == nil {
		t.Fatal("expected duplicate key error")
	}

	first, err := svc.Assign(ctx, accountID, "home-cta")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	second, err := svc.Assign(ctx, accountID, "home-cta")
	if err != nil || first.Variant != second.Variant {
		t.Fatalf("variant not stable: %+v %+v err=%v", first, second, err)
	}
	var exposures int
	if err := pool.QueryRow(ctx,
		`select count(*) from experiment_exposure where experiment_id = $1`, created.ID).
		Scan(&exposures); err != nil {
		t.Fatalf("exposure count: %v", err)
	}
	if exposures != 1 {
		t.Fatalf("exposures = %d, want 1", exposures)
	}

	if VariantFor("home-cta", accountID, 100) != "treatment" {
		t.Fatal("100% traffic should be treatment")
	}
	if VariantFor("home-cta", accountID, 0) != "control" {
		t.Fatal("0% traffic should be control")
	}

	items, err := svc.List(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("list = %+v err=%v", items, err)
	}
	if items[0].ExposureA+items[0].ExposureB != 1 {
		t.Fatalf("unexpected exposure counts: %+v", items[0])
	}
}
