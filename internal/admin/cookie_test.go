package admin

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func newTestService(ttl time.Duration) *Service {
	return NewService(nil, nil, "test-secret-0123456789-0123456789", ttl, slog.Default())
}

func testToken(t *testing.T, s *Service, id, username string) string {
	t.Helper()
	token, err := s.sign(adminRecord{ID: id, Username: username, DisplayName: username},
		[]string{"admin"}, []string{"*"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func TestAdminCookieAttributes(t *testing.T) {
	s := newTestService(8 * time.Hour)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/v1/admin/me", nil)

	s.setAdminCookie(rec, req, "token-value")

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != adminCookieName {
		t.Errorf("cookie name = %q, want %q", cookie.Name, adminCookieName)
	}
	if cookie.Value != "token-value" {
		t.Errorf("cookie value = %q, want %q", cookie.Value, "token-value")
	}
	if !cookie.HttpOnly {
		t.Error("cookie should be HttpOnly")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie SameSite = %v, want Strict", cookie.SameSite)
	}
	if cookie.Path != "/v1" {
		t.Errorf("cookie Path = %q, want %q", cookie.Path, "/v1")
	}
	if want := int((8 * time.Hour).Seconds()); cookie.MaxAge != want {
		t.Errorf("cookie MaxAge = %d, want %d", cookie.MaxAge, want)
	}
	if cookie.Secure {
		t.Error("cookie should not be Secure over plain HTTP")
	}
}

func TestAdminCookieSecure(t *testing.T) {
	s := newTestService(time.Hour)

	t.Run("TLS", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "https://example.com/v1/admin/me", nil)
		s.setAdminCookie(rec, req, "token")
		if cookie := rec.Result().Cookies()[0]; !cookie.Secure {
			t.Error("cookie should be Secure over TLS")
		}
	})

	t.Run("X-Forwarded-Proto", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/v1/admin/me", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		s.setAdminCookie(rec, req, "token")
		if cookie := rec.Result().Cookies()[0]; !cookie.Secure {
			t.Error("cookie should be Secure when X-Forwarded-Proto is https")
		}
	})
}

func TestAuthBearerTakesPrecedenceOverCookie(t *testing.T) {
	s := newTestService(time.Hour)
	bearer := testToken(t, s, "u-bearer", "bearer")
	cookieToken := testToken(t, s, "u-cookie", "cookie")

	var got Principal
	handler := s.Auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = From(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: cookieToken})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got.Username != "bearer" {
		t.Errorf("authenticated as %q, want bearer", got.Username)
	}
}

func TestAuthBearerDoesNotRequireCustomHeader(t *testing.T) {
	s := newTestService(time.Hour)
	token := testToken(t, s, "u-bearer", "bearer")

	handler := s.Auth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/questions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAuthCookieWriteRequiresCustomHeader(t *testing.T) {
	s := newTestService(time.Hour)
	token := testToken(t, s, "u-cookie", "cookie")

	handler := s.Auth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/questions", nil)
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/admin/questions", nil)
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: token})
	req.Header.Set(adminRequestHeader, "1")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status with custom header = %d, want 200", rec.Code)
	}
}

func TestAuthCookieReadDoesNotRequireCustomHeader(t *testing.T) {
	s := newTestService(time.Hour)
	token := testToken(t, s, "u-cookie", "cookie")

	handler := s.Auth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/me", nil)
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestLogoutClearsCookieIdempotently(t *testing.T) {
	s := newTestService(time.Hour)
	router := chi.NewRouter()
	s.Routes(router)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/admin/auth/logout", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want 200", i+1, rec.Code)
		}
		cookies := rec.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("attempt %d: expected 1 cookie, got %d", i+1, len(cookies))
		}
		cookie := cookies[0]
		if cookie.Name != adminCookieName {
			t.Errorf("attempt %d: cookie name = %q, want %q", i+1, cookie.Name, adminCookieName)
		}
		if cookie.Value != "" {
			t.Errorf("attempt %d: cookie value = %q, want empty", i+1, cookie.Value)
		}
		if cookie.MaxAge >= 0 {
			t.Errorf("attempt %d: cookie MaxAge = %d, want negative", i+1, cookie.MaxAge)
		}
		if cookie.Path != "/v1" {
			t.Errorf("attempt %d: cookie Path = %q, want %q", i+1, cookie.Path, "/v1")
		}
	}
}
