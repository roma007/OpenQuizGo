package identity_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/httpapi"
	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/platform/cryptox"
	"github.com/roma007/OpenQuizGo/internal/rule"
)

// TestTVLoginFlow 覆盖 081：电视登录、令牌作用域隔离、按电视配置与孩子指定。
func TestTVLoginFlow(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	redisURL := os.Getenv("TEST_REDIS_URL")
	if dbURL == "" || redisURL == "" {
		t.Skip("TEST_DATABASE_URL / TEST_REDIS_URL not set")
	}

	ctx := context.Background()
	prepareDatabase(t, dbURL)
	rdb := prepareRedis(t, redisURL)
	defer func() { _ = rdb.Close() }()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect pool: %v", err)
	}
	defer pool.Close()

	cipher, err := cryptox.New("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", "test-phone-hash-key")
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	identitySvc := identity.NewService(pool, rdb, cipher, identity.Options{
		JWTSecret:   "test-jwt-secret-at-least-32-characters",
		AccessTTL:   15 * time.Minute,
		RefreshTTL:  time.Hour,
		SMSMode:     "mock",
		SMSMockCode: "123456",
		Environment: "test",
	}, logger)
	ruleSvc := rule.New(pool, identitySvc, logger)

	server := httptest.NewServer(httpapi.New(httpapi.Deps{
		Logger:   logger,
		DB:       pool,
		Redis:    rdb,
		Identity: identitySvc,
		Rule:     ruleSvc,
		Version:  "test",
		Env:      "test",
	}))
	defer server.Close()

	c := &client{t: t, base: server.URL + "/v1"}

	// 家长登录并设置 PIN（后面改电视配置要用）。
	parentPhone := uniquePhone()
	c.expect(http.MethodPost, "/auth/sms/code", map[string]string{"phone": parentPhone}, "", http.StatusOK, nil)
	var parent loginResponse
	c.expect(http.MethodPost, "/auth/login",
		map[string]string{"phone": parentPhone, "code": "123456"}, "", http.StatusOK, &parent)
	c.expect(http.MethodPost, "/family/pin", map[string]string{"new_pin": "1234"}, parent.AccessToken, http.StatusOK, nil)

	// 电视端用同一个手机号 + 短信验证码登录，不经过激活码。
	// 短信有 60 秒频控，这里直接写入验证码，模拟家长在电视上重新收取。
	phoneHash := cipher.Hash(parentPhone)
	if err := rdb.Set(ctx, "sms:login:"+phoneHash, "123456", 5*time.Minute).Err(); err != nil {
		t.Fatalf("seed sms code: %v", err)
	}
	var tvLogin identity.TVLoginResult
	c.expect(http.MethodPost, "/tv/login", map[string]any{
		"phone":       parentPhone,
		"code":        "123456",
		"device_id":   "tv-flow-living",
		"device_name": "客厅电视",
	}, "", http.StatusOK, &tvLogin)
	if tvLogin.DeviceToken == "" || tvLogin.Device.ID == "" {
		t.Fatalf("tv login response incomplete: %+v", tvLogin)
	}
	if !tvLogin.ExpiresAt.After(time.Now().Add(300 * 24 * time.Hour)) {
		t.Fatalf("device token ttl too short: %v", tvLogin.ExpiresAt)
	}

	// 作用域隔离：设备令牌不能进家长接口，家长令牌不能当设备令牌用。
	c.expect(http.MethodGet, "/family/tvs", nil, tvLogin.DeviceToken, http.StatusUnauthorized, nil)
	c.expect(http.MethodGet, "/tv/me", nil, parent.AccessToken, http.StatusForbidden, nil)

	// 家长端看到这台电视，默认配置：做题开、上限开、15 分钟、60 分钟、未指定孩子。
	var tvs []identity.TVDevice
	c.expect(http.MethodGet, "/family/tvs", nil, parent.AccessToken, http.StatusOK, &tvs)
	if len(tvs) != 1 {
		t.Fatalf("tvs = %d, want 1", len(tvs))
	}
	tv := tvs[0]
	if tv.ID != tvLogin.Device.ID || tv.Name != "客厅电视" {
		t.Fatalf("unexpected tv: %+v", tv)
	}
	if !tv.Config.QuizEnabled || !tv.Config.LimitEnabled ||
		tv.Config.UnlockMinutes != 15 || tv.Config.DailyLimit != 60 ||
		tv.Config.HardStopGrace != 10 || tv.Config.QuizScope != "random" {
		t.Fatalf("unexpected default config: %+v", tv.Config)
	}
	if tv.ChildID == "" || tv.GradeTag != "" {
		t.Fatalf("placeholder child expected without grade: %+v", tv)
	}

	// 电视自己看上下文：未指定孩子时按全库随机出题。
	var tvCtx rule.TVContextResult
	c.expect(http.MethodGet, "/tv/context", nil, tvLogin.DeviceToken, http.StatusOK, &tvCtx)
	if !tvCtx.Allowed || tvCtx.HardStop || tvCtx.Reason != "ok" {
		t.Fatalf("unexpected tv context: %+v", tvCtx)
	}
	if !tvCtx.QuizEnabled || tvCtx.QuizScope != "random" || tvCtx.DailyLimit != 60 {
		t.Fatalf("context should reflect tv config: %+v", tvCtx)
	}

	// 家长改这台电视的配置：做题关、上限开、强制 20 分钟、每日 90 分钟、放行 5 分钟。
	cfg := tv.Config
	cfg.QuizEnabled = false
	cfg.LimitEnabled = true
	cfg.UnlockMinutes = 20
	cfg.DailyLimit = 90
	cfg.HardStopGrace = 5
	var updated identity.TVConfig
	c.expect(http.MethodPut, "/family/tvs/"+tv.ID+"/config", map[string]any{
		"pin":    "1234",
		"config": cfg,
	}, parent.AccessToken, http.StatusOK, &updated)
	if updated.QuizEnabled || updated.UnlockMinutes != 20 ||
		updated.DailyLimit != 90 || updated.HardStopGrace != 5 {
		t.Fatalf("config not applied: %+v", updated)
	}

	// 错误 PIN 被拒。
	c.expect(http.MethodPut, "/family/tvs/"+tv.ID+"/config", map[string]any{
		"pin":    "0000",
		"config": updated,
	}, parent.AccessToken, http.StatusForbidden, nil)

	// 配置按电视隔离：别的家庭读不到。
	otherPhone := uniquePhone()
	c.expect(http.MethodPost, "/auth/sms/code", map[string]string{"phone": otherPhone}, "", http.StatusOK, nil)
	var other loginResponse
	c.expect(http.MethodPost, "/auth/login",
		map[string]string{"phone": otherPhone, "code": "123456"}, "", http.StatusOK, &other)
	c.expect(http.MethodGet, "/family/tvs/"+tv.ID+"/config", nil,
		other.AccessToken, http.StatusNotFound, nil)
	var otherTVs []identity.TVDevice
	c.expect(http.MethodGet, "/family/tvs", nil, other.AccessToken, http.StatusOK, &otherTVs)
	if len(otherTVs) != 0 {
		t.Fatalf("other family should not see this tv: %d", len(otherTVs))
	}

	// 指定孩子：昵称 + 年级。
	var assigned identity.TVDevice
	c.expect(http.MethodPut, "/family/tvs/"+tv.ID+"/child", map[string]string{
		"nickname":  "小明",
		"grade_tag": "g3",
	}, parent.AccessToken, http.StatusOK, &assigned)
	if assigned.ChildName != "小明" || assigned.GradeTag != "g3" {
		t.Fatalf("assign child failed: %+v", assigned)
	}

	// 电视端 /tv/me 返回同一台设备。
	var me identity.TVDevice
	c.expect(http.MethodGet, "/tv/me", nil, tvLogin.DeviceToken, http.StatusOK, &me)
	if me.ID != tv.ID || me.ChildName != "小明" {
		t.Fatalf("tv me mismatch: %+v", me)
	}

	// 解绑后设备令牌立刻失效。
	c.expect(http.MethodDelete, "/family/tvs/"+tv.ID, nil, parent.AccessToken, http.StatusOK, nil)
	c.expect(http.MethodGet, "/tv/me", nil, tvLogin.DeviceToken, http.StatusForbidden, nil)
}
