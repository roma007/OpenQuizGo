package watch

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
	svc, err := New(pool, ruleSvc, nil, "test-secret-0123456789-0123456789", "", logger)
	if err != nil {
		t.Fatal(err)
	}

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

func TestDiagnosticsUploadAndList(t *testing.T) {
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
	svc, err := New(pool, rule.New(pool, nil, logger), nil,
		"test-secret-0123456789-0123456789", "", logger)
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.DevicePrincipal{DeviceID: "tv-diag", ChildID: childID}

	entry, err := svc.UploadDiagnostics(ctx, principal, "line-1\nline-2\n", " 测试上传 ")
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if entry.ID == 0 || entry.Bytes == 0 || entry.Note != "测试上传" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	items, err := svc.ListDiagnostics(ctx, "tv-diag", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].DeviceID != "tv-diag" {
		t.Fatalf("unexpected list: %+v", items)
	}
	detail, err := svc.Diagnostic(ctx, entry.ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if !strings.Contains(detail.Content, "line-2") {
		t.Fatalf("detail content = %q", detail.Content)
	}

	if _, err := svc.UploadDiagnostics(ctx, principal, "", ""); err == nil {
		t.Fatal("expected empty content error")
	}
	tooLarge := strings.Repeat("x", maxDiagnosticBytes+1)
	if _, err := svc.UploadDiagnostics(ctx, principal, tooLarge, ""); err == nil {
		t.Fatal("expected oversize error")
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
		device_diagnostic,
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

func assertGrantStatus(t *testing.T, pool *pgxpool.Pool, grantID, want string) {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(),
		`select status from watch_grant where id = $1`, grantID).Scan(&status); err != nil {
		t.Fatalf("grant status: %v", err)
	}
	if status != want {
		t.Fatalf("grant %s status = %s, want %s", grantID, status, want)
	}
}

func TestAlertsListAndRead(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `
		insert into tamper_event (device_id, child_id, type, detail)
		values ('tv-alert', $1, 'permission_revoked', '{"permission":"accessibility"}'::jsonb)`,
		childID); err != nil {
		t.Fatalf("insert tamper: %v", err)
	}
	var accountID string
	if err := pool.QueryRow(ctx, `
		select fm.account_id from child_profile cp
		join family_member fm on fm.family_id = cp.family_id
		where cp.id = $1`, childID).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}

	repo := newRepository(pool)
	alerts, unread, err := repo.listAlerts(ctx, accountID, 10)
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if len(alerts) != 1 || alerts[0].Type != "permission_revoked" || alerts[0].Read {
		t.Fatalf("unexpected alerts: %+v", alerts)
	}
	if unread != 1 {
		t.Fatalf("unread = %d, want 1", unread)
	}

	if err := repo.markAlertsRead(ctx, accountID); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	alerts, unread, err = repo.listAlerts(ctx, accountID, 10)
	if err != nil {
		t.Fatalf("list after read: %v", err)
	}
	if unread != 0 || !alerts[0].Read {
		t.Fatalf("after read unread=%d alert=%+v", unread, alerts[0])
	}
}

func TestIssueParentGrant(t *testing.T) {
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
	repo := newRepository(pool)
	now := time.Now()
	start := now.Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)

	grant, err := repo.issueParent(ctx, childID, "tv-parent", 15, 60, start, end, now, "parent-jti-1")
	if err != nil {
		t.Fatalf("issue parent: %v", err)
	}
	if grant.Minutes != 15 || grant.SessionID != "" || grant.Status != "active" {
		t.Fatalf("unexpected grant: %+v", grant)
	}

	var source string
	if err := pool.QueryRow(ctx,
		`select source from watch_grant where id = $1`, grant.ID).Scan(&source); err != nil {
		t.Fatalf("source: %v", err)
	}
	if source != "parent" {
		t.Fatalf("source = %s, want parent", source)
	}

	// 旧 15 分钟授权被替换并释放预留，请求 60 分钟应得到完整 60 分钟。
	capped, err := repo.issueParent(ctx, childID, "tv-parent", 60, 60, start, end, now, "parent-jti-2")
	if err != nil {
		t.Fatalf("issue capped: %v", err)
	}
	if capped.Minutes != 60 {
		t.Fatalf("capped minutes = %d, want 60", capped.Minutes)
	}
	// 同孩子同设备重复签发后，旧授权被撤销，仅保留最新一条 active。
	assertGrantStatus(t, pool, grant.ID, "revoked")
	var parentActive int
	if err := pool.QueryRow(ctx, `
		select count(*) from watch_grant
		where child_id = $1 and device_id = $2 and status = 'active'`,
		childID, "tv-parent").Scan(&parentActive); err != nil {
		t.Fatalf("count parent active: %v", err)
	}
	if parentActive != 1 {
		t.Fatalf("parent active grants = %d, want 1", parentActive)
	}

	// 不同设备的授权不受影响（使用另一个孩子避免额度已用尽）。
	childID3 := seedWatchChild(t, pool, "hash-watch-3")
	otherA, err := repo.issueParent(ctx, childID3, "tv-parent-a", 10, 60, start, end, now, "parent-jti-4")
	if err != nil {
		t.Fatalf("issue other device a: %v", err)
	}
	otherB, err := repo.issueParent(ctx, childID3, "tv-parent-b", 20, 60, start, end, now, "parent-jti-5")
	if err != nil {
		t.Fatalf("issue other device b: %v", err)
	}
	assertGrantStatus(t, pool, otherA.ID, "active")
	assertGrantStatus(t, pool, otherB.ID, "active")

	// 每日额度 10 分钟时，请求 30 分钟应截断到 10（新孩子）。
	childID2 := seedWatchChild(t, pool, "hash-watch-2")
	limited, err := repo.issueParent(ctx, childID2, "tv-parent-2", 30, 10, start, end, now, "parent-jti-3")
	if err != nil {
		t.Fatalf("issue limited: %v", err)
	}
	if limited.Minutes != 10 {
		t.Fatalf("limited minutes = %d, want 10", limited.Minutes)
	}

	if err := repo.insertAudit(ctx, "account-1", "tv-parent", grant.ID, grant.Minutes, "127.0.0.1"); err != nil {
		t.Fatalf("audit: %v", err)
	}
	var actions int
	if err := pool.QueryRow(ctx,
		`select count(*) from audit_log where action = 'device.unlock'`).Scan(&actions); err != nil {
		t.Fatalf("audit count: %v", err)
	}
	if actions != 1 {
		t.Fatalf("audit rows = %d, want 1", actions)
	}
}

func TestIssueReplacesPreviousActiveGrant(t *testing.T) {
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
	svc, err := New(pool, rule.New(pool, nil, logger), nil,
		"test-secret-0123456789-0123456789", "", logger)
	if err != nil {
		t.Fatal(err)
	}

	ruleView := RuleView{UnlockMinutes: 20, DailyLimit: 60, Timezone: "Asia/Shanghai"}
	principal := identity.DevicePrincipal{DeviceID: "tv-replace", ChildID: childID}
	session1 := insertSession(t, pool, childID, principal.DeviceID)
	grant1, err := svc.Issue(ctx, IssueRequest{
		SessionID: session1,
		ChildID:   childID,
		DeviceID:  principal.DeviceID,
		Rule:      ruleView,
	})
	if err != nil {
		t.Fatalf("issue first: %v", err)
	}

	// 连续第二次签发后，仅最新授权为 active。
	session2 := insertSession(t, pool, childID, principal.DeviceID)
	grant2, err := svc.Issue(ctx, IssueRequest{
		SessionID: session2,
		ChildID:   childID,
		DeviceID:  principal.DeviceID,
		Rule:      ruleView,
	})
	if err != nil {
		t.Fatalf("issue second: %v", err)
	}
	if grant2.GrantID == grant1.GrantID {
		t.Fatalf("second issue should create a new grant")
	}
	assertGrantStatus(t, pool, grant1.GrantID, "revoked")
	assertGrantStatus(t, pool, grant2.GrantID, "active")
	var sameDeviceActive int
	if err := pool.QueryRow(ctx, `
		select count(*) from watch_grant
		where child_id = $1 and device_id = $2 and status = 'active'`,
		childID, principal.DeviceID).Scan(&sameDeviceActive); err != nil {
		t.Fatalf("count active: %v", err)
	}
	if sameDeviceActive != 1 {
		t.Fatalf("same device active grants = %d, want 1", sameDeviceActive)
	}
	active, err := svc.ActiveGrant(ctx, principal)
	if err != nil {
		t.Fatalf("active grant: %v", err)
	}
	if active.GrantID != grant2.GrantID {
		t.Fatalf("active grant = %s, want %s", active.GrantID, grant2.GrantID)
	}

	// 其他设备的 active 授权不受影响。
	otherPrincipal := identity.DevicePrincipal{DeviceID: "tv-replace-other", ChildID: childID}
	otherSession := insertSession(t, pool, childID, otherPrincipal.DeviceID)
	otherGrant, err := svc.Issue(ctx, IssueRequest{
		SessionID: otherSession,
		ChildID:   childID,
		DeviceID:  otherPrincipal.DeviceID,
		Rule:      ruleView,
	})
	if err != nil {
		t.Fatalf("issue other device: %v", err)
	}

	// 再次在第一个设备签发，只撤销该设备的旧授权。
	session3 := insertSession(t, pool, childID, principal.DeviceID)
	grant3, err := svc.Issue(ctx, IssueRequest{
		SessionID: session3,
		ChildID:   childID,
		DeviceID:  principal.DeviceID,
		Rule:      ruleView,
	})
	if err != nil {
		t.Fatalf("issue third: %v", err)
	}
	assertGrantStatus(t, pool, grant2.GrantID, "revoked")
	assertGrantStatus(t, pool, grant3.GrantID, "active")
	assertGrantStatus(t, pool, otherGrant.GrantID, "active")

	// 重放旧 session 返回旧记录，且不得撤销当前 active 授权。
	replay, err := svc.Issue(ctx, IssueRequest{
		SessionID: session1,
		ChildID:   childID,
		DeviceID:  principal.DeviceID,
		Rule:      ruleView,
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if replay.GrantID != grant1.GrantID {
		t.Fatalf("replay grant = %s, want %s", replay.GrantID, grant1.GrantID)
	}
	assertGrantStatus(t, pool, grant3.GrantID, "active")
	assertGrantStatus(t, pool, otherGrant.GrantID, "active")
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
	svc, err := New(pool, rule.New(pool, nil, logger), nil, "test-secret-0123456789-0123456789", "", logger)
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.DevicePrincipal{DeviceID: "tv-concurrent", ChildID: childID}

	// 预先消耗 2 分钟，当日仅剩 1 分钟：新授权会替换旧 active，
	// 因此 100 个并发请求仍只能成功签发 1 条，避免超过每日额度。
	if _, err := pool.Exec(ctx, `
		insert into watch_log (child_id, device_id, start_at, end_at, watched_seconds)
		values ($1, $2, now(), now(), 120)`, childID, principal.DeviceID); err != nil {
		t.Fatal(err)
	}

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
	// 替换语义下，后续请求会撤销上一条 active 并释放其预留，
	// 因此并发请求可以全部成功，但最终只能保留一条 active。
	if limited != 0 {
		t.Fatalf("success=%d limited=%d, want no limit errors", success, limited)
	}

	var activeCount, activeMinutes int
	if err := pool.QueryRow(ctx, `
		select count(*), coalesce(sum(minutes), 0)
		from watch_grant
		where child_id = $1 and device_id = $2 and status = 'active'`,
		childID, principal.DeviceID).Scan(&activeCount, &activeMinutes); err != nil {
		t.Fatal(err)
	}
	if activeCount != 1 {
		t.Fatalf("active grants = %d, want 1", activeCount)
	}

	var usedSeconds int
	if err := pool.QueryRow(ctx, `
		select coalesce(sum(watched_seconds), 0)::int from watch_log
		where child_id = $1`, childID).Scan(&usedSeconds); err != nil {
		t.Fatal(err)
	}
	if usedSeconds+activeMinutes*60 > 3*60 {
		t.Fatalf("used %ds + active %dmin exceeds daily limit 3min", usedSeconds, activeMinutes)
	}
}

func TestOfflineLease(t *testing.T) {
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
	svc, err := New(pool, ruleSvc, nil, "test-secret-0123456789-0123456789", "", logger)
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.DevicePrincipal{DeviceID: "tv-offline", ChildID: childID}

	lease, err := svc.IssueOfflineLease(ctx, principal)
	if err != nil {
		t.Fatalf("issue lease: %v", err)
	}
	if lease.MaxMinutes != 10 || lease.Token == "" || lease.PublicKey == "" {
		t.Fatalf("unexpected lease: %+v", lease)
	}

	publicKey, err := base64.StdEncoding.DecodeString(lease.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := jwt.Parse(lease.Token, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != "EdDSA" {
			return nil, errors.New("unexpected alg")
		}
		return ed25519.PublicKey(publicKey), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("verify lease token: %v", err)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["typ"] != "offline_lease" || claims["device_id"] != principal.DeviceID {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	now := time.Now()
	result, err := svc.ReportOffline(ctx, principal, OfflineReportRequest{
		LeaseID: lease.LeaseID,
		Entries: []OfflineEntry{{
			StartAt:        now.Add(-4 * time.Minute),
			EndAt:          now.Add(-1 * time.Second),
			WatchedSeconds: 239,
			PausedSeconds:  5,
			TargetApp:      "com.example.video",
		}},
	})
	if err != nil {
		t.Fatalf("report offline: %v", err)
	}
	if result.ReportedMinutes != 4 || result.Clamped {
		t.Fatalf("unexpected report: %+v", result)
	}

	var watched int
	if err := pool.QueryRow(ctx, `
		select coalesce(sum(watched_seconds), 0)::int from watch_log
		where child_id = $1 and target_id = 'com.example.video'`, childID).Scan(&watched); err != nil {
		t.Fatal(err)
	}
	if watched != 239 {
		t.Fatalf("watched = %d, want 239", watched)
	}

	_, err = svc.ReportOffline(ctx, principal, OfflineReportRequest{
		LeaseID: lease.LeaseID,
		Entries: []OfflineEntry{{
			StartAt: now.Add(-2 * time.Minute), EndAt: now, WatchedSeconds: 60,
		}},
	})
	reportedErr, ok := err.(*Error)
	if !ok || reportedErr.Code != "LEASE_ALREADY_REPORTED" {
		t.Fatalf("expected already reported, got %v", err)
	}

	// 超过租约额度的部分被截断。
	child2 := seedWatchChild(t, pool, "hash-offline-2")
	if _, err := pool.Exec(ctx,
		`update watch_rule set offline_fallback_min = 1 where child_id = $1`, child2); err != nil {
		t.Fatal(err)
	}
	principal2 := identity.DevicePrincipal{DeviceID: "tv-offline-2", ChildID: child2}
	lease2, err := svc.IssueOfflineLease(ctx, principal2)
	if err != nil {
		t.Fatalf("issue lease 2: %v", err)
	}
	if lease2.MaxMinutes != 1 {
		t.Fatalf("max minutes = %d, want 1", lease2.MaxMinutes)
	}
	now2 := time.Now()
	clamped, err := svc.ReportOffline(ctx, principal2, OfflineReportRequest{
		LeaseID: lease2.LeaseID,
		Entries: []OfflineEntry{{
			StartAt: now2.Add(-4 * time.Minute), EndAt: now2,
			WatchedSeconds: 240, TargetApp: "com.example.video",
		}},
	})
	if err != nil {
		t.Fatalf("clamped report: %v", err)
	}
	if !clamped.Clamped || clamped.ReportedMinutes != 1 {
		t.Fatalf("unexpected clamped result: %+v", clamped)
	}
}
