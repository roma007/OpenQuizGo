package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// Principal 是管理端登录主体。
type Principal struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Has 判断是否具备权限。
func (p Principal) Has(permission string) bool {
	for _, item := range p.Permissions {
		if item == permission || item == "*" {
			return true
		}
	}
	return false
}

type claims struct {
	Type     string   `json:"typ"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	Perms    []string `json:"perms"`
	jwt.RegisteredClaims
}

// Service 提供管理端登录与 RBAC 中间件。
type Service struct {
	pool   *pgxpool.Pool
	redis  *redis.Client
	secret []byte
	ttl    time.Duration
	logger *slog.Logger
}

// NewService 构造管理端认证服务。
func NewService(pool *pgxpool.Pool, rdb *redis.Client, secret string, ttl time.Duration, logger *slog.Logger) *Service {
	return &Service{pool: pool, redis: rdb, secret: []byte(secret), ttl: ttl, logger: logger}
}

type adminRecord struct {
	ID           string
	Username     string
	PasswordHash string
	DisplayName  string
	Status       string
}

// Login 校验用户名密码并签发管理员令牌。
func (s *Service) Login(ctx context.Context, username, password string) (Principal, string, error) {
	username = strings.TrimSpace(username)
	key := "rl:admin:login:" + username
	if count, _ := s.redis.Get(ctx, key).Int(); count >= 5 {
		return Principal{}, "", errRateLimited
	}

	var rec adminRecord
	err := s.pool.QueryRow(ctx, `
		select id, username, password_hash, display_name, status
		from admin_user where username = $1`, username).Scan(
		&rec.ID, &rec.Username, &rec.PasswordHash, &rec.DisplayName, &rec.Status)
	if err != nil || rec.Status != "active" ||
		bcrypt.CompareHashAndPassword([]byte(rec.PasswordHash), []byte(password)) != nil {
		s.recordFailure(ctx, key)
		return Principal{}, "", errInvalidCredentials
	}
	_ = s.redis.Del(ctx, key).Err()

	roles, permissions, err := s.permissions(ctx, rec.ID)
	if err != nil {
		return Principal{}, "", err
	}

	expiresAt := time.Now().Add(s.ttl)
	token, err := s.sign(rec, roles, permissions, expiresAt)
	if err != nil {
		return Principal{}, "", err
	}
	return Principal{
		ID:          rec.ID,
		Username:    rec.Username,
		DisplayName: rec.DisplayName,
		Roles:       roles,
		Permissions: permissions,
		ExpiresAt:   expiresAt,
	}, token, nil
}

func (s *Service) recordFailure(ctx context.Context, key string) {
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return
	}
	if count == 1 {
		_ = s.redis.Expire(ctx, key, 15*time.Minute).Err()
	}
}

func (s *Service) permissions(ctx context.Context, userID string) ([]string, []string, error) {
	roleRows, err := s.pool.Query(ctx, `
		select r.code from admin_role r
		join admin_user_role ur on ur.role_id = r.id
		where ur.user_id = $1 order by r.code`, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("query roles: %w", err)
	}
	defer roleRows.Close()
	var roles []string
	for roleRows.Next() {
		var code string
		if err := roleRows.Scan(&code); err != nil {
			return nil, nil, err
		}
		roles = append(roles, code)
	}

	permRows, err := s.pool.Query(ctx, `
		select distinct p.code from admin_permission p
		join admin_role_permission rp on rp.permission_id = p.id
		join admin_user_role ur on ur.role_id = rp.role_id
		where ur.user_id = $1 order by p.code`, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("query permissions: %w", err)
	}
	defer permRows.Close()
	var permissions []string
	for permRows.Next() {
		var code string
		if err := permRows.Scan(&code); err != nil {
			return nil, nil, err
		}
		permissions = append(permissions, code)
	}
	return roles, permissions, nil
}

func (s *Service) sign(rec adminRecord, roles, permissions []string, expiresAt time.Time) (string, error) {
	now := time.Now()
	payload := claims{
		Type:     "admin",
		Username: rec.Username,
		Roles:    roles,
		Perms:    permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   rec.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        randomString(10),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, payload).SignedString(s.secret)
}

func (s *Service) parse(raw string) (Principal, error) {
	payload := &claims{}
	token, err := jwt.ParseWithClaims(raw, payload, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid || payload.Type != "admin" || payload.Subject == "" {
		return Principal{}, errInvalidCredentials
	}
	return Principal{
		ID:          payload.Subject,
		Username:    payload.Username,
		Roles:       payload.Roles,
		Permissions: payload.Perms,
		ExpiresAt:   payload.ExpiresAt.Time,
	}, nil
}

const (
	// adminCookieName 是管理端令牌 Cookie 名。
	adminCookieName = "openquizgo_admin"
	// adminRequestHeader 用于标记来自管理端前端自身的写请求，配合 SameSite 防 CSRF。
	adminRequestHeader = "X-Admin-Request"
)

// Auth 校验管理员令牌，优先 Bearer，其次 HttpOnly Cookie。
func (s *Service) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token != "" {
			principal, err := s.parse(token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "管理员令牌无效或已过期")
				return
			}
			// 显示名与最新权限在登录时已固化到令牌中，令牌有效期 8 小时。
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, principal)))
			return
		}

		cookie, err := r.Cookie(adminCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "管理员令牌无效或已过期")
			return
		}
		principal, err := s.parse(cookie.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "管理员令牌无效或已过期")
			return
		}
		// Cookie 鉴权的写请求必须带自定义头，Bearer 客户端不受此限制。
		if !isSafeMethod(r.Method) && r.Header.Get(adminRequestHeader) != "1" {
			writeError(w, http.StatusForbidden, "CSRF_REQUIRED", "缺少必要的请求头")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, principal)))
	})
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// isSecureRequest 判断请求是否经由 HTTPS（含反向代理）。
func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// adminCookie 构造管理端令牌 Cookie；maxAge 为秒，负数表示立即失效。
func adminCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     adminCookieName,
		Value:    value,
		Path:     "/v1",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	}
}

func (s *Service) setAdminCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, adminCookie(r, token, int(s.ttl.Seconds())))
}

func (s *Service) clearAdminCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, adminCookie(r, "", -1))
}

// Require 要求指定权限。
func (s *Service) Require(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := From(r.Context())
			if !ok || !principal.Has(permission) {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "缺少权限："+permission)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type principalKeyType int

const principalKey principalKeyType = iota

// From 读取当前管理员。
func From(ctx context.Context) (Principal, bool) {
	v, ok := ctx.Value(principalKey).(Principal)
	return v, ok
}

// Routes 注册管理端认证路由。
func (s *Service) Routes(r chi.Router) {
	r.Post("/admin/auth/login", s.handleLogin)
	r.Post("/admin/auth/logout", s.handleLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.Auth)
		r.Get("/admin/me", s.handleMe)
	})
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求体不合法")
		return
	}
	principal, token, err := s.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) {
			writeError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			return
		}
		s.logger.Error("admin login", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "服务端错误")
		return
	}
	s.setAdminCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_in": int64(s.ttl.Seconds()),
		"admin":      principal,
	})
}

// handleLogout 清除管理端 Cookie，幂等且不要求已登录。
func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearAdminCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := From(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "未登录")
		return
	}
	writeJSON(w, http.StatusOK, principal)
}

/* ------------------------------ 错误与响应 ------------------------------ */

type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string { return e.Message }

var (
	errInvalidCredentials = &apiError{Status: http.StatusUnauthorized, Code: "AUTH_INVALID_CREDENTIALS", Message: "用户名或密码错误"}
	errRateLimited        = &apiError{Status: http.StatusTooManyRequests, Code: "RATE_LIMITED", Message: "登录失败次数过多，请稍后再试"}
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(dst)
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
