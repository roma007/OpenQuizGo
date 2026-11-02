package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testRouter() http.Handler {
	return New(Deps{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version: "test",
		Env:     "test",
	})
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Request-Id"); got == "" {
		t.Error("X-Request-Id header is missing")
	}
}

func TestMeta(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/meta", nil)
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["version"] != "test" {
		t.Errorf("version = %q, want test", body["version"])
	}
}

func TestRequestIDPassthrough(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "trace-123")
	testRouter().ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got != "trace-123" {
		t.Errorf("X-Request-Id = %q, want trace-123", got)
	}
}

func TestNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", body.Code)
	}
}
