package watch

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
	"github.com/roma007/OpenQuizGo/internal/rule"
	"github.com/roma007/OpenQuizGo/migrations"
)

func TestGrantAndHeartbeat(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareWatchDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	childID := seedWatchChild(t, pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ruleSvc := rule.New(pool, nil, logger)
	svc := New(pool, ruleSvc, "test-secret-0123456789-0123456789", logger)

	principal := identity.DevicePrincipal{DeviceID: "tv-watch", ChildID: childID}
	sessionID := insertSession(t, pool, childID, principal.DeviceID)

	grant, err := svc.Issue(ctx, IssueRequest{
		SessionID: sessionID,
		ChildID:   childID,
		DeviceID:  principal.DeviceID,
		Rule:      RuleView{UnlockMinutes: 20, DailyLimit: 60, Timezone: "Asia/Shanghai"},
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if grant.Minutes != 20 || grant.LeaseToken == "" || grant.RemainingSeconds != 1200 {
		t.Fatalf("unexpected grant: %+v", grant)
	}

	again, err := svc.Issue(ctx, IssueRequest{
		SessionID: sessionID,
		ChildID:   childID,
		DeviceID:  principal.DeviceID,
		Rule:      RuleView{UnlockMinutes: 20, DailyLimit: 60, Timezone: "Asia/Shanghai"},
	})
	if err != nil || again.GrantID != grant.GrantID {
		t.Fatalf("issue should be idempotent: %+v %v", again, err)
	}

	active, err := svc.ActiveGrant(ctx, principal)
	if err != nil {
		t.Fatalf("active: %v", err)
	}
	if active.GrantID != grant.GrantID {
		t.Fatalf("active grant = %s, want %s", active.GrantID, grant.GrantID)
	}

	shiftHeartbeat(t, pool, grant.GrantID, 30*time.Second)
	beat, err := svc.Heartbeat(ctx, principal, grant.GrantID, HeartbeatRequest{})
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if beat.ConsumedSeconds < 29 || beat.ConsumedSeconds > 31 {
		t.Fatalf("consumed = %d, want ~30", beat.ConsumedSeconds)
	}

	shiftHeartbeat(t, pool, grant.GrantID, 60*time.Second)
	paused, err := svc.Heartbeat(ctx, principal, grant.GrantID, HeartbeatRequest{Paused: true})
	if err != nil {
		t.Fatalf("paused heartbeat: %v", err)
	}
	if paused.ConsumedSeconds != beat.ConsumedSeconds {
		t.Fatalf("paused consumed = %d, want %d", paused.ConsumedSeconds, beat.ConsumedSeconds)
	}

	// 答题会话进行中，即使客户端未标记暂停也不计费。
	insertActiveSession(t, pool, childID, principal.DeviceID)
	shiftHeartbeat(t, pool, grant.GrantID, 30*time.Second)
	quizPaused, err := svc.Heartbeat(ctx, principal, grant.GrantID, HeartbeatRequest{})
	if err != nil {
		t.Fatalf("quiz heartbeat: %v", err)
	}
	if quizPaused.ConsumedSeconds != beat.ConsumedSeconds || !quizPaused.Paused {
		t.Fatalf("quiz period should not accrue: %+v", quizPaused)
	}

	// 时段外心跳吊销授权。
	now := time.Now()
	start := now.Add(2 * time.Hour).Format("15:04")
	end := now.Add(2*time.Hour + time.Minute).Format("15:04")
	if _, err := pool.Exec(ctx, `
		update watch_rule set time_window = jsonb_build_object('allow', jsonb_build_array(
			jsonb_build_object('start', $2::text, 'end', $3::text)))
		where child_id = $1`, childID, start, end); err != nil {
		t.Fatalf("update window: %v", err)
	}
	revoked, err := svc.Heartbeat(ctx, principal, grant.GrantID, HeartbeatRequest{})
	if err != nil {
		t.Fatalf("revoke heartbeat: %v", err)
	}
	if !revoked.Revoked || revoked.Status != "revoked" {
		t.Fatalf("expected revoked, got %+v", revoked)
	}

	// 每日额度不足时拒绝签发。
	child2 := seedWatchChild(t, pool, "hash-watch-2")
	session2 := insertSession(t, pool, child2, "tv-watch-2")
	if _, err := pool.Exec(ctx,
		`update watch_rule set daily_limit = 1 where child_id = $1`, child2); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		insert into watch_log (child_id, device_id, start_at, end_at, watched_seconds)
		values ($1, 'tv-watch-2', now(), now(), 60)`, child2); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Issue(ctx, IssueRequest{
		SessionID: session2,
		ChildID:   child2,
		DeviceID:  "tv-watch-2",
		Rule:      RuleView{UnlockMinutes: 20, DailyLimit: 1, Timezone: "Asia/Shanghai"},
	})
	watchErr, ok := err.(*Error)
	if !ok || watchErr.Code != "RULE_DAILY_LIMIT" {
		t.Fatalf("expected daily limit error, got %v", err)
	}

	if err := svc.ReportCapability(ctx, principal, Capability{
		GuardMode:   "standard",
		Permissions: map[string]bool{"accessibility": true},
		OSVersion:   "14",
	}); err != nil {
		t.Fatalf("capability: %v", err)
	}
	if err := svc.ReportTamper(ctx, principal, TamperEvent{
		Type:   "permission_revoked",
		Detail: map[string]any{"permission": "accessibility"},
	}); err != nil {
		t.Fatalf("tamper: %v", err)
	}
}

func prepareWatchDB(t *testing.T, url string) {
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
		watch_log, watch_grant, quiz_session_item, quiz_session,
		watch_rule, child_profile, family_member, family, parent_account, audit_log
		cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func seedWatchChild(t *testing.T, pool *pgxpool.Pool, hash ...string) string {
	t.Helper()
	ctx := context.Background()
	phoneHash := "hash-watch"
	if len(hash) > 0 {
		phoneHash = hash[0]
	}
	var accountID, familyID, childID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', $1)
		returning id`, phoneHash).Scan(&accountID); err != nil {
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
		values ($1, '计量孩子', 'low') returning id`, familyID).Scan(&childID); err != nil {
		t.Fatalf("child: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into watch_rule (child_id, unlock_minutes, pass_threshold, subject_weight,
			fun_ratio_cap, fun_enabled, time_window, daily_limit, offline_fallback_min,
			scope_mode, allowed_apps, weight_preset, timezone, version)
		values ($1, 20, 3, '{"math":2,"chinese":1,"fun":0.5}'::jsonb, 2, true,
		        '{"allow":[]}'::jsonb, 60, 10, 'media', '[]'::jsonb,
		        'balanced', 'Asia/Shanghai', 1)`, childID); err != nil {
		t.Fatalf("rule: %v", err)
	}
	return childID
}

func insertSession(t *testing.T, pool *pgxpool.Pool, childID, deviceID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		insert into quiz_session (child_id, device_id, rule_version, state,
			question_count, pass_threshold, score, correct_count, rule_snapshot,
			expires_at, finished_at)
		values ($1, $2, 1, 'passed', 5, 3, 4, 2, '{}'::jsonb, now() + interval '1 minute', now())
		returning id`, childID, deviceID).Scan(&id); err != nil {
		t.Fatalf("session: %v", err)
	}
	return id
}

func insertActiveSession(t *testing.T, pool *pgxpool.Pool, childID, deviceID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		insert into quiz_session (child_id, device_id, rule_version, state,
			question_count, pass_threshold, score, correct_count, rule_snapshot, expires_at)
		values ($1, $2, 1, 'active', 5, 3, 0, 0, '{}'::jsonb, now() + interval '5 minutes')`,
		childID, deviceID); err != nil {
		t.Fatalf("active session: %v", err)
	}
}

func shiftHeartbeat(t *testing.T, pool *pgxpool.Pool, grantID string, d time.Duration) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		update watch_grant set last_heartbeat_at = now() - make_interval(secs => $2) where id = $1`,
		grantID, int(d.Seconds())); err != nil {
		t.Fatalf("shift heartbeat: %v", err)
	}
}

func TestConcurrentIssueRespectsDailyLimit(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareWatchDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	childID := seedWatchChild(t, pool)
	if _, err := pool.Exec(ctx,
		`update watch_rule set daily_limit = 3, unlock_minutes = 1 where child_id = $1`, childID); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := New(pool, rule.New(pool, nil, logger), "test-secret-0123456789-0123456789", logger)
	principal := identity.DevicePrincipal{DeviceID: "tv-concurrent", ChildID: childID}

	sessions := make([]string, 100)
	for i := range sessions {
		sessions[i] = insertSession(t, pool, childID, principal.DeviceID)
	}

	type result struct {
		grant Grant
		err   error
	}
	results := make(chan result, len(sessions))
	for _, sessionID := range sessions {
		go func(id string) {
			grant, err := svc.Issue(ctx, IssueRequest{
				SessionID: id,
				ChildID:   childID,
				DeviceID:  principal.DeviceID,
				Rule:      RuleView{UnlockMinutes: 1, DailyLimit: 3, Timezone: "Asia/Shanghai"},
			})
			results <- result{grant: grant, err: err}
		}(sessionID)
	}

	success, limited := 0, 0
	for range sessions {
		res := <-results
		if res.err == nil {
			success++
			continue
		}
		if watchErr, ok := res.err.(*Error); ok && watchErr.Code == "RULE_DAILY_LIMIT" {
			limited++
			continue
		}
		t.Fatalf("unexpected error: %v", res.err)
	}
	if success != 3 || limited != 97 {
		t.Fatalf("success=%d limited=%d, want 3/97", success, limited)
	}

	var totalMinutes int
	if err := pool.QueryRow(ctx,
		`select coalesce(sum(minutes), 0) from watch_grant where child_id = $1`, childID).Scan(&totalMinutes); err != nil {
		t.Fatal(err)
	}
	if totalMinutes != 3 {
		t.Fatalf("total granted minutes = %d, want 3", totalMinutes)
	}
}
