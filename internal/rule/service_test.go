package rule

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/platform/cryptox"
	"github.com/roma007/OpenQuizGo/migrations"
)

func TestRuleService(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareRuleDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	accountID, childID := seedFamily(t, pool)
	insertRule(t, pool, childID)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cipher, err := cryptox.New("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	identitySvc := identity.NewService(pool, nil, cipher, identity.Options{
		JWTSecret:   "test-secret-0123456789-0123456789",
		AccessTTL:   time.Minute,
		RefreshTTL:  time.Hour,
		SMSMode:     "mock",
		SMSMockCode: "123456",
		Environment: "test",
	}, logger)
	if err := identitySvc.SetPIN(ctx, accountID, "", "1234"); err != nil {
		t.Fatalf("set pin: %v", err)
	}

	svc := New(pool, identitySvc, logger)
	r, version, err := svc.Get(ctx, accountID, childID)
	if err != nil {
		t.Fatalf("get rule: %v", err)
	}
	if version != 1 || r.PassThreshold != 3 {
		t.Fatalf("unexpected rule: %+v v%d", r, version)
	}

	r.PassThreshold = 4
	if _, _, err := svc.Update(ctx, accountID, childID, "0000", r, ""); err == nil {
		t.Fatal("expected wrong pin error")
	}
	updated, version, err := svc.Update(ctx, accountID, childID, "1234", r, "")
	if err != nil {
		t.Fatalf("update rule: %v", err)
	}
	if version != 2 || updated.PassThreshold != 4 {
		t.Fatalf("unexpected update: %+v v%d", updated, version)
	}

	preview, err := svc.Preview(ctx, accountID, childID, updated)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.N < 3 || preview.MaxScore < updated.PassThreshold {
		t.Fatalf("unexpected preview: %+v", preview)
	}

	bad := updated
	bad.PassThreshold = 100
	if _, err := svc.Preview(ctx, accountID, childID, bad); err == nil {
		t.Fatal("expected unreachable threshold error")
	}

	now := time.Now()
	windowStart := now.Add(2 * time.Hour).Format("15:04")
	windowEnd := now.Add(2*time.Hour + time.Minute).Format("15:04")
	blocked := updated
	blocked.TimeWindow = TimeWindow{Allow: []Window{{Start: windowStart, End: windowEnd}}}
	if _, _, err := svc.Update(ctx, accountID, childID, "1234", blocked, ""); err != nil {
		t.Fatalf("update window: %v", err)
	}
	if _, _, err := svc.CanStart(ctx, childID); err == nil {
		t.Fatal("expected time window rejection")
	}

	limited := updated
	limited.TimeWindow = TimeWindow{}
	limited.DailyLimit = 1
	if _, _, err := svc.Update(ctx, accountID, childID, "1234", limited, ""); err != nil {
		t.Fatalf("update daily: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into watch_log (child_id, device_id, start_at, end_at, watched_seconds)
		values ($1, 'tv-rule', now(), now(), 120)`, childID); err != nil {
		t.Fatalf("insert watch log: %v", err)
	}
	if _, _, err := svc.CanStart(ctx, childID); err == nil {
		t.Fatal("expected daily limit rejection")
	}
	state, err := svc.Context(ctx, childID)
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	if state.Allowed || state.Reason != "daily_limit" || state.RemainingMinutes != 0 {
		t.Fatalf("unexpected context: %+v", state)
	}

	var auditCount int
	if err := pool.QueryRow(ctx,
		`select count(*) from audit_log where action = 'rule.update' and target_id = $1`,
		childID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Fatalf("audit count = %d, want 3", auditCount)
	}
}

func prepareRuleDB(t *testing.T, url string) {
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
		watch_log, watch_grant, answer_log, quiz_session_item, quiz_session,
		watch_rule, child_profile, family_member, family, parent_account, audit_log
		cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func seedFamily(t *testing.T, pool *pgxpool.Pool) (accountID, childID string) {
	t.Helper()
	ctx := context.Background()
	var familyID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'hash-rule-test')
		returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`insert into family (owner_id) values ($1) returning id`, accountID).Scan(&familyID); err != nil {
		t.Fatalf("family: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into family_member (family_id, account_id, role, second_factor_verified)
		values ($1, $2, 'owner', true)`, familyID, accountID); err != nil {
		t.Fatalf("member: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		insert into child_profile (family_id, nickname, stage_code)
		values ($1, '规则孩子', 'low') returning id`, familyID).Scan(&childID); err != nil {
		t.Fatalf("child: %v", err)
	}
	return accountID, childID
}

func insertRule(t *testing.T, pool *pgxpool.Pool, childID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		insert into watch_rule (
			child_id, unlock_minutes, pass_threshold, subject_weight, fun_ratio_cap,
			fun_enabled, time_window, daily_limit, offline_fallback_min, scope_mode,
			allowed_apps, weight_preset, timezone, version)
		values ($1, 20, 3, '{"math":2,"chinese":1,"science":1,"fun":0.5}'::jsonb, 2,
		        true, '{"allow":[]}'::jsonb, 60, 10, 'media', '[]'::jsonb,
		        'balanced', 'Asia/Shanghai', 1)`, childID)
	if err != nil {
		t.Fatalf("insert rule: %v", err)
	}
}
