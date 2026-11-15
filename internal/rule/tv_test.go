package rule

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/platform/cryptox"
)

// seedTV 在已 seed 的家庭里插入一台登录电视与它的配置。
func seedTV(t *testing.T, pool *pgxpool.Pool, accountID, childID string, mutate func(*identity.TVConfig)) (devicePK, deviceID string) {
	t.Helper()
	ctx := context.Background()
	deviceID = "tv-" + childID[:8]
	if err := pool.QueryRow(ctx, `
		insert into tv_device (family_id, child_id, device_id, name)
		select family_id, $1, $2, '客厅电视' from child_profile where id = $1
		returning id`, childID, deviceID).Scan(&devicePK); err != nil {
		t.Fatalf("insert tv device: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into tv_config (device_pk) values ($1)`, devicePK); err != nil {
		t.Fatalf("insert tv config: %v", err)
	}
	if mutate != nil {
		cfg := defaultTVConfig()
		mutate(&cfg)
		if _, err := pool.Exec(ctx, `
			update tv_config
			set quiz_enabled = $2, limit_enabled = $3, unlock_minutes = $4,
			    daily_limit = $5, subjects = $6::jsonb, pass_threshold = $7,
			    quiz_scope = $8, hard_stop_grace = $9
			where device_pk = $1`,
			devicePK, cfg.QuizEnabled, cfg.LimitEnabled, cfg.UnlockMinutes,
			cfg.DailyLimit, `[]`, cfg.PassThreshold, cfg.QuizScope,
			cfg.HardStopGrace); err != nil {
			t.Fatalf("update tv config: %v", err)
		}
	}
	return devicePK, deviceID
}

func defaultTVConfig() identity.TVConfig {
	return identity.TVConfig{
		QuizEnabled:   true,
		LimitEnabled:  true,
		UnlockMinutes: 15,
		DailyLimit:    60,
		PassThreshold: 3,
		QuizScope:     "random",
		HardStopGrace: 10,
	}
}

func tvPrincipal(devicePK, deviceID, childID string) identity.DevicePrincipal {
	return identity.DevicePrincipal{
		Kind:     "tv",
		DevicePK: devicePK,
		DeviceID: deviceID,
		ChildID:  childID,
	}
}

// usedSeconds 写入今天的观看流水，用于把当日额度耗尽。
func usedSeconds(t *testing.T, pool *pgxpool.Pool, childID, deviceID string, seconds int) {
	t.Helper()
	now := time.Now()
	if _, err := pool.Exec(context.Background(), `
		insert into watch_log (child_id, device_id, start_at, end_at, watched_seconds)
		values ($1, $2, $3, $4, $5)`,
		childID, deviceID, now.Add(-time.Duration(seconds)*time.Second), now, seconds); err != nil {
		t.Fatalf("insert watch log: %v", err)
	}
}

func TestTVConfigAndContext(t *testing.T) {
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

	devicePK, deviceID := seedTV(t, pool, accountID, childID, nil)
	principal := tvPrincipal(devicePK, deviceID, childID)

	// 默认值来自 tv_config（不是 watch_rule）。
	cfg, err := svc.GetTVConfig(ctx, accountID, devicePK)
	if err != nil {
		t.Fatalf("get tv config: %v", err)
	}
	if !cfg.QuizEnabled || !cfg.LimitEnabled || cfg.UnlockMinutes != 15 ||
		cfg.DailyLimit != 60 || cfg.HardStopGrace != 10 || cfg.QuizScope != "random" {
		t.Fatalf("unexpected default config: %+v", cfg)
	}

	// 别的账号读不到这台电视。
	if _, err := svc.GetTVConfig(ctx, "00000000-0000-0000-0000-000000000000", devicePK); err == nil {
		t.Fatal("expected cross-family config access to fail")
	}

	// 修改需要 PIN，且带乐观锁。
	if _, err := svc.UpdateTVConfig(ctx, accountID, devicePK, "0000", cfg); err == nil {
		t.Fatal("expected wrong pin to fail")
	}
	next := cfg
	next.UnlockMinutes = 20
	next.Subjects = []string{"math"}
	updated, err := svc.UpdateTVConfig(ctx, accountID, devicePK, "1234", next)
	if err != nil {
		t.Fatalf("update tv config: %v", err)
	}
	if updated.UnlockMinutes != 20 || updated.Version != cfg.Version+1 || len(updated.Subjects) != 1 {
		t.Fatalf("unexpected updated config: %+v", updated)
	}
	stale := cfg
	if _, err := svc.UpdateTVConfig(ctx, accountID, devicePK, "1234", stale); err == nil {
		t.Fatal("expected version conflict")
	}

	// 上下文：默认允许、按电视的配置回报。
	state, err := svc.ContextForDevice(ctx, principal)
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	if !state.Allowed || state.HardStop || state.Reason != "ok" {
		t.Fatalf("unexpected context: %+v", state)
	}
	if state.UnlockMinutes != 20 || state.QuizScope != "random" {
		t.Fatalf("context should reflect tv config: %+v", state)
	}

	// 两个开关独立：关掉做题，上限仍在 → 允许看，但不能发起答题。
	cfg = updated
	if updated, err = svc.UpdateTVConfig(ctx, accountID, devicePK, "1234", func() identity.TVConfig {
		c := cfg
		c.QuizEnabled = false
		c.LimitEnabled = true
		return c
	}()); err != nil {
		t.Fatalf("disable quiz: %v", err)
	}
	if _, err := svc.CanStartDevice(ctx, principal); err == nil {
		t.Fatal("expected quiz disabled")
	} else if ruleErr, ok := err.(*Error); !ok || ruleErr.Code != "QUIZ_DISABLED" {
		t.Fatalf("unexpected error: %v", err)
	}
	if state, err = svc.ContextForDevice(ctx, principal); err != nil || !state.Allowed || state.HardStop {
		t.Fatalf("limit should still allow watching: %+v err=%v", state, err)
	}

	// 关掉上限，做题仍开 → 额度耗尽也不硬停，但答题照旧。
	cfg = updated
	if updated, err = svc.UpdateTVConfig(ctx, accountID, devicePK, "1234", func() identity.TVConfig {
		c := cfg
		c.QuizEnabled = true
		c.LimitEnabled = false
		return c
	}()); err != nil {
		t.Fatalf("disable limit: %v", err)
	}
	usedSeconds(t, pool, childID, deviceID, 3600)
	state, err = svc.ContextForDevice(ctx, principal)
	if err != nil {
		t.Fatalf("context after limit disabled: %v", err)
	}
	if state.HardStop || !state.Allowed {
		t.Fatalf("limit disabled should never hard stop: %+v", state)
	}
	if plan, err := svc.CanStartDevice(ctx, principal); err != nil || plan.Random != true {
		t.Fatalf("quiz should still start: %+v err=%v", plan, err)
	}

	// 打开上限且额度耗尽 → 硬停；作答被拒。
	cfg = updated
	if updated, err = svc.UpdateTVConfig(ctx, accountID, devicePK, "1234", func() identity.TVConfig {
		c := cfg
		c.QuizEnabled = true
		c.LimitEnabled = true
		return c
	}()); err != nil {
		t.Fatalf("enable limit: %v", err)
	}
	state, err = svc.ContextForDevice(ctx, principal)
	if err != nil {
		t.Fatalf("context after limit: %v", err)
	}
	if !state.HardStop || state.Allowed || state.Reason != "daily_limit" {
		t.Fatalf("expected hard stop: %+v", state)
	}
	if _, err := svc.CanStartDevice(ctx, principal); err == nil {
		t.Fatal("expected daily limit to block quiz start")
	} else if ruleErr, ok := err.(*Error); !ok || ruleErr.Code != "DAILY_LIMIT_REACHED" {
		t.Fatalf("unexpected error: %v", err)
	}

	// 家长放行期间不再硬停，即使当日额度已用完。
	if _, err := pool.Exec(ctx, `
		insert into watch_grant (child_id, device_id, minutes, valid_from, valid_until, jti, status)
		values ($1, $2, 10, now(), now() + interval '10 minutes', 'jti-tv-grace', 'active')`,
		childID, deviceID); err != nil {
		t.Fatalf("insert grant: %v", err)
	}
	state, err = svc.ContextForDevice(ctx, principal)
	if err != nil {
		t.Fatalf("context with grant: %v", err)
	}
	if state.HardStop || !state.Allowed || state.Reason != "grant" || state.GrantUntil == nil {
		t.Fatalf("active grant should lift hard stop: %+v", state)
	}
}

func TestValidateTVConfig(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*identity.TVConfig)
		wantErr bool
	}{
		{"默认合法", func(c *identity.TVConfig) {}, false},
		{"放行 1 分钟", func(c *identity.TVConfig) { c.HardStopGrace = 1 }, false},
		{"放行 120 分钟", func(c *identity.TVConfig) { c.HardStopGrace = 120 }, false},
		{"放行 0 分钟", func(c *identity.TVConfig) { c.HardStopGrace = 0 }, false}, // 归一化为 10
		{"放行 121 分钟", func(c *identity.TVConfig) { c.HardStopGrace = 121 }, true},
		{"间隔 0", func(c *identity.TVConfig) { c.UnlockMinutes = 0 }, false}, // 归一化为 15
		{"间隔 241", func(c *identity.TVConfig) { c.UnlockMinutes = 241 }, true},
		{"上限 721", func(c *identity.TVConfig) { c.DailyLimit = 721 }, true},
		{"门槛 0", func(c *identity.TVConfig) { c.PassThreshold = 0 }, false}, // 归一化为 3
		{"出题模式非法", func(c *identity.TVConfig) { c.QuizScope = "all" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultTVConfig()
			tc.mutate(&cfg)
			err := validateTVConfig(normalizeTVConfig(cfg))
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestApplyTVConfig(t *testing.T) {
	base := Normalize(Rule{
		UnlockMinutes: 20,
		PassThreshold: 3,
		SubjectWeight: map[string]float64{"math": 1.5, "chinese": 1.5, "fun": 0.5},
		DailyLimit:    90,
		FunEnabled:    true,
		FunRatioCap:   2,
	})
	cfg := defaultTVConfig()
	cfg.UnlockMinutes = 5
	cfg.DailyLimit = 30
	cfg.Subjects = []string{"math"}
	cfg.PassThreshold = 6

	out := applyTVConfig(base, cfg)
	if out.UnlockMinutes != 5 || out.DailyLimit != 30 || out.PassThreshold != 6 {
		t.Fatalf("config not applied: %+v", out)
	}
	if _, ok := out.SubjectWeight["chinese"]; ok {
		t.Fatalf("unselected subject should be dropped: %+v", out.SubjectWeight)
	}
	if out.SubjectWeight["math"] != 1.5 {
		t.Fatalf("selected subject weight changed: %+v", out.SubjectWeight)
	}

	// 关闭上限时不限制观看时长（用大额度表示）。
	cfg.LimitEnabled = false
	if out := applyTVConfig(base, cfg); out.DailyLimit != 720 {
		t.Fatalf("limit disabled daily_limit = %d, want 720", out.DailyLimit)
	}

	// 未选学科时保留孩子规则里的权重。
	cfg.Subjects = nil
	if out := applyTVConfig(base, cfg); len(out.SubjectWeight) != len(base.SubjectWeight) {
		t.Fatalf("weights should stay when no subjects selected: %+v", out.SubjectWeight)
	}
}
