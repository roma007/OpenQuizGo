package identity_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"

	"github.com/roma007/OpenQuizGo/internal/httpapi"
	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/platform/cryptox"
	"github.com/roma007/OpenQuizGo/migrations"
)

func TestIdentityFlow(t *testing.T) {
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
	svc := identity.NewService(pool, rdb, cipher, identity.Options{
		JWTSecret:   "test-jwt-secret-at-least-32-characters",
		AccessTTL:   15 * time.Minute,
		RefreshTTL:  time.Hour,
		SMSMode:     "mock",
		SMSMockCode: "123456",
		Environment: "test",
	}, logger)

	server := httptest.NewServer(httpapi.New(httpapi.Deps{
		Logger:   logger,
		DB:       pool,
		Redis:    rdb,
		Identity: svc,
		Version:  "test",
		Env:      "test",
	}))
	defer server.Close()

	c := &client{t: t, base: server.URL + "/v1"}

	phone := uniquePhone()
	c.expect(http.MethodPost, "/auth/sms/code", map[string]string{"phone": phone}, "", http.StatusOK, nil)
	var login loginResponse
	c.expect(http.MethodPost, "/auth/login",
		map[string]string{"phone": phone, "code": "123456"}, "", http.StatusOK, &login)
	if login.AccessToken == "" || login.RefreshToken == "" || login.Account.FamilyID == "" {
		t.Fatalf("login response incomplete: %+v", login)
	}

	var me identity.Account
	c.expect(http.MethodGet, "/family/me", nil, login.AccessToken, http.StatusOK, &me)
	if me.ID != login.Account.ID {
		t.Fatalf("me id = %s, want %s", me.ID, login.Account.ID)
	}

	var child identity.Child
	c.expect(http.MethodPost, "/family/children", map[string]any{
		"nickname":    "小明",
		"stage":       "low",
		"birth_year":  2018,
		"birth_month": 6,
	}, login.AccessToken, http.StatusCreated, &child)
	if child.Stage != "low" || child.Nickname != "小明" {
		t.Fatalf("unexpected child: %+v", child)
	}

	var code identity.ActivationCode
	deviceID := fmt.Sprintf("tv-test-%d", time.Now().UnixNano())
	c.expect(http.MethodPost, "/device/activate/code",
		map[string]string{"device_id": deviceID, "platform": "tv"}, "", http.StatusOK, &code)
	if code.Code == "" {
		t.Fatal("empty activation code")
	}

	c.expect(http.MethodPost, "/family/devices/activate",
		map[string]string{"code": code.Code, "child_id": child.ID}, login.AccessToken, http.StatusOK, nil)

	var deviceToken identity.DeviceToken
	c.expect(http.MethodPost, "/device/token",
		map[string]string{"code": code.Code, "device_id": deviceID}, "", http.StatusOK, &deviceToken)
	if deviceToken.Token == "" || deviceToken.ChildID != child.ID {
		t.Fatalf("unexpected device token: %+v", deviceToken)
	}

	var deviceMe struct {
		DeviceID  string `json:"device_id"`
		ChildID   string `json:"child_id"`
		ChildName string `json:"child_name"`
	}
	c.expect(http.MethodGet, "/device/me", nil, deviceToken.Token, http.StatusOK, &deviceMe)
	if deviceMe.ChildID != child.ID {
		t.Fatalf("device me child = %s, want %s", deviceMe.ChildID, child.ID)
	}

	// 设备端自助解绑：第二台设备解绑后令牌立即失效。
	deviceID2 := fmt.Sprintf("tv-self-%d", time.Now().UnixNano())
	var code2 identity.ActivationCode
	c.expect(http.MethodPost, "/device/activate/code",
		map[string]string{"device_id": deviceID2, "platform": "tv"}, "", http.StatusOK, &code2)
	c.expect(http.MethodPost, "/family/devices/activate",
		map[string]string{"code": code2.Code, "child_id": child.ID}, login.AccessToken, http.StatusOK, nil)
	var token2 identity.DeviceToken
	c.expect(http.MethodPost, "/device/token",
		map[string]string{"code": code2.Code, "device_id": deviceID2}, "", http.StatusOK, &token2)
	c.expect(http.MethodGet, "/device/me", nil, token2.Token, http.StatusOK, nil)
	c.expect(http.MethodDelete, "/device/me", nil, token2.Token, http.StatusOK, nil)
	c.expect(http.MethodGet, "/device/me", nil, token2.Token, http.StatusForbidden, nil)

	var devices []identity.Device
	c.expect(http.MethodGet, "/family/devices", nil, login.AccessToken, http.StatusOK, &devices)
	if len(devices) != 1 || devices[0].DeviceID != deviceID {
		t.Fatalf("unexpected devices: %+v", devices)
	}

	c.expect(http.MethodDelete, "/family/devices/"+devices[0].ID, nil, login.AccessToken, http.StatusOK, nil)
	c.expect(http.MethodGet, "/device/me", nil, deviceToken.Token, http.StatusForbidden, nil)

	c.expect(http.MethodPost, "/family/pin", map[string]string{"new_pin": "1234"},
		login.AccessToken, http.StatusOK, nil)
	c.expect(http.MethodPost, "/family/pin/verify", map[string]string{"pin": "1234"},
		login.AccessToken, http.StatusOK, nil)
	c.expect(http.MethodPost, "/family/pin/verify", map[string]string{"pin": "0000"},
		login.AccessToken, http.StatusForbidden, nil)

	var invite identity.Invite
	c.expect(http.MethodPost, "/family/members/invite", nil, login.AccessToken, http.StatusOK, &invite)

	phone2 := uniquePhone()
	c.expect(http.MethodPost, "/auth/sms/code", map[string]string{"phone": phone2}, "", http.StatusOK, nil)
	var login2 loginResponse
	c.expect(http.MethodPost, "/auth/login",
		map[string]string{"phone": phone2, "code": "123456"}, "", http.StatusOK, &login2)
	c.expect(http.MethodPost, "/family/members/join",
		map[string]string{"code": invite.Code}, login2.AccessToken, http.StatusOK, nil)

	var members []identity.Member
	c.expect(http.MethodGet, "/family/members", nil, login2.AccessToken, http.StatusOK, &members)
	if len(members) != 2 {
		t.Fatalf("members = %d, want 2", len(members))
	}
	var children []identity.Child
	c.expect(http.MethodGet, "/family/children", nil, login2.AccessToken, http.StatusOK, &children)
	if len(children) != 1 {
		t.Fatalf("invited parent children = %d, want 1", len(children))
	}

	var refreshed loginResponse
	c.expect(http.MethodPost, "/auth/refresh",
		map[string]string{"refresh_token": login.RefreshToken}, "", http.StatusOK, &refreshed)
	if refreshed.AccessToken == "" || refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("refresh token should rotate")
	}
}

type loginResponse struct {
	Account      identity.Account `json:"account"`
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	ExpiresIn    int64            `json:"expires_in"`
}

type client struct {
	t    *testing.T
	base string
}

func (c *client) expect(method, path string, body any, token string, want int, out any) {
	c.t.Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)

	if want != 0 && resp.StatusCode != want {
		c.t.Fatalf("%s %s status = %d, want %d, body = %s", method, path, resp.StatusCode, want, payload)
	}
	if out != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, out); err != nil {
			c.t.Fatalf("decode %s %s: %v, body = %s", method, path, err, payload)
		}
	}
}

func prepareDatabase(t *testing.T, url string) {
	t.Helper()
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("open sql: %v", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if _, err := db.Exec(`truncate table
		parent_session, family_invite, device_bind, watch_rule, child_profile,
		family_member, family, parent_account
		cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func prepareRedis(t *testing.T, url string) *redis.Client {
	t.Helper()
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse redis: %v", err)
	}
	client := redis.NewClient(opts)
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}
	return client
}

func uniquePhone() string {
	return fmt.Sprintf("139%08d", time.Now().UnixNano()%100000000)
}
