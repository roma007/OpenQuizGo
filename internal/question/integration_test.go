package question_test

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
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/roma007/OpenQuizGo/internal/admin"
	"github.com/roma007/OpenQuizGo/internal/httpapi"
	"github.com/roma007/OpenQuizGo/internal/question"
	"github.com/roma007/OpenQuizGo/migrations"
)

func TestQuestionWorkflow(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	redisURL := os.Getenv("TEST_REDIS_URL")
	if dbURL == "" || redisURL == "" {
		t.Skip("TEST_DATABASE_URL / TEST_REDIS_URL not set")
	}

	ctx := context.Background()
	prepareDB(t, dbURL)
	rdb := prepareRedis(t, redisURL)
	defer func() { _ = rdb.Close() }()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	createAdmin(t, pool, "admin", "admin12345", "super_admin")
	createAdmin(t, pool, "editor", "editor12345", "editor")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	adminSvc := admin.NewService(pool, rdb, "test-secret-0123456789-0123456789", 8*time.Hour, logger)
	questionSvc := question.New(pool, logger)

	server := httptest.NewServer(httpapi.New(httpapi.Deps{
		Logger:    logger,
		DB:        pool,
		Redis:     rdb,
		Admin:     adminSvc,
		Questions: questionSvc,
		Version:   "test",
		Env:       "test",
	}))
	defer server.Close()

	c := &client{t: t, base: server.URL + "/v1"}
	adminToken := c.login("admin", "admin12345")

	var created question.Question
	c.expect(http.MethodPost, "/admin/questions", validQuestion("1+1=?"), adminToken, http.StatusCreated, &created)
	if created.Status != "draft" || created.Version != 1 {
		t.Fatalf("unexpected created question: %+v", created)
	}

	c.expect(http.MethodPost, "/admin/questions/"+created.ID+"/publish", nil, adminToken, http.StatusConflict, nil)
	c.expect(http.MethodPost, "/admin/questions/"+created.ID+"/submit-review", nil, adminToken, http.StatusOK, nil)
	c.expect(http.MethodPost, "/admin/questions/"+created.ID+"/review",
		map[string]bool{"approve": true}, adminToken, http.StatusOK, nil)
	c.expect(http.MethodPost, "/admin/questions/"+created.ID+"/publish", nil, adminToken, http.StatusOK, nil)
	c.expect(http.MethodPut, "/admin/questions/"+created.ID, validQuestion("2+2=?"), adminToken, http.StatusConflict, nil)

	bad := validQuestion("bad")
	bad["category"] = "core"
	bad["subject"] = "fun"
	c.expect(http.MethodPost, "/admin/questions", bad, adminToken, http.StatusUnprocessableEntity, nil)

	var listed question.ListResult
	c.expect(http.MethodGet, "/admin/questions?subject=math&stage=low&difficulty=2&type=choice",
		nil, adminToken, http.StatusOK, &listed)
	if listed.Total != 1 || listed.Items[0].Status != "online" {
		t.Fatalf("unexpected list: %+v", listed)
	}

	importBody := map[string]any{"questions": importQuestions(100)}
	var imported question.ImportResult
	c.expect(http.MethodPost, "/admin/questions/import", importBody, adminToken, http.StatusOK, &imported)
	if imported.Imported != 100 {
		t.Fatalf("imported = %d, want 100", imported.Imported)
	}

	invalid := map[string]any{"questions": []any{map[string]any{
		"subject": "math", "category": "core", "stage": "low",
		"difficulty": 9, "type": "choice", "stem": "bad", "options": []any{}, "answer": []any{},
	}}}
	c.expect(http.MethodPost, "/admin/questions/import", invalid, adminToken, http.StatusUnprocessableEntity, nil)

	editorToken := c.login("editor", "editor12345")
	c.expect(http.MethodPost, "/admin/questions/"+created.ID+"/publish", nil, editorToken, http.StatusForbidden, nil)

	var audit []question.AuditEntry
	c.expect(http.MethodGet, "/admin/audit?limit=20", nil, adminToken, http.StatusOK, &audit)
	if len(audit) == 0 {
		t.Fatal("audit log is empty")
	}

	var filtered []question.AuditEntry
	c.expect(http.MethodGet,
		"/admin/audit?target_type=question&action=question.create&limit=20",
		nil, adminToken, http.StatusOK, &filtered)
	if len(filtered) != 1 || filtered[0].Action != "question.create" ||
		filtered[0].TargetType != "question" {
		t.Fatalf("unexpected filtered audit: %+v", filtered)
	}

	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	var todayAudit []question.AuditEntry
	c.expect(http.MethodGet, "/admin/audit?from="+today+"&to="+today,
		nil, adminToken, http.StatusOK, &todayAudit)
	if len(todayAudit) == 0 {
		t.Fatal("today audit is empty")
	}
	var tomorrowAudit []question.AuditEntry
	c.expect(http.MethodGet, "/admin/audit?from="+tomorrow,
		nil, adminToken, http.StatusOK, &tomorrowAudit)
	if len(tomorrowAudit) != 0 {
		t.Fatalf("tomorrow audit = %d, want 0", len(tomorrowAudit))
	}

	req, _ := http.NewRequest(http.MethodGet,
		c.base+"/admin/audit/export?action=question.create", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("export audit: %v", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK ||
		!strings.Contains(string(payload), "actor_type") ||
		!strings.Contains(string(payload), "question.create") {
		t.Fatalf("unexpected audit csv: status=%d body=%s", resp.StatusCode, payload)
	}
}

type client struct {
	t    *testing.T
	base string
}

func (c *client) login(username, password string) string {
	c.t.Helper()
	var resp struct {
		Token string `json:"token"`
	}
	c.expect(http.MethodPost, "/admin/auth/login",
		map[string]string{"username": username, "password": password}, "", http.StatusOK, &resp)
	if resp.Token == "" {
		c.t.Fatalf("login %s returned empty token", username)
	}
	return resp.Token
}

func (c *client) expect(method, path string, body any, token string, want int, out any) {
	c.t.Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal: %v", err)
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
	if resp.StatusCode != want {
		c.t.Fatalf("%s %s status = %d, want %d, body = %s", method, path, resp.StatusCode, want, payload)
	}
	if out != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, out); err != nil {
			c.t.Fatalf("decode: %v, body = %s", err, payload)
		}
	}
}

func validQuestion(stem string) map[string]any {
	return map[string]any{
		"subject": "math", "category": "core", "stage": "low",
		"grade_tag": "g2", "difficulty": 2, "type": "choice", "stem": stem,
		"options": []map[string]string{{"id": "a", "text": "1"}, {"id": "b", "text": "2"}},
		"answer":  []string{"b"},
	}
}

func importQuestions(n int) []map[string]any {
	items := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, map[string]any{
			"subject": "math", "category": "core", "stage": "low",
			"difficulty": i%5 + 1, "type": "choice",
			"stem":    fmt.Sprintf("导入题 %d", i+1),
			"options": []map[string]string{{"id": "a", "text": "1"}, {"id": "b", "text": "2"}},
			"answer":  []string{"a"},
		})
	}
	return items
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
	if _, err := db.Exec(`truncate table question, topic_pack, audit_log, admin_user cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func prepareRedis(t *testing.T, url string) *redis.Client {
	t.Helper()
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("redis url: %v", err)
	}
	client := redis.NewClient(opts)
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}
	return client
}

func createAdmin(t *testing.T, pool *pgxpool.Pool, username, password, role string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ctx := context.Background()
	var userID string
	if err := pool.QueryRow(ctx, `
		insert into admin_user (username, password_hash, display_name)
		values ($1, $2, $1) returning id`, username, string(hash)).Scan(&userID); err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into admin_user_role (user_id, role_id)
		select $1, id from admin_role where code = $2`, userID, role); err != nil {
		t.Fatalf("assign role: %v", err)
	}
}
