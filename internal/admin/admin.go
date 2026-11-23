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
	"unicode/utf8"

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
	// Epoch 是签发时的令牌版本，改密后递增即可让旧令牌立即失效。
	Epoch int64 `json:"-"`
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
	Epoch    int64    `json:"ep,omitempty"`
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
	epoch := s.tokenEpoch(ctx, rec.ID)
	token, err := s.sign(rec, roles, permissions, expiresAt, epoch)
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
		Epoch:       epoch,
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

// passwordFailLimit 是改密接口的失败次数上限（15 分钟窗口）。
const passwordFailLimit = 5

// ChangePassword 校验当前口令并更新为新口令，随后递增令牌版本使所有旧会话立即失效。
func (s *Service) ChangePassword(ctx context.Context, adminID, current, next string) error {
	failKey := "rl:admin:passwd:" + adminID
	if s.redis != nil {
		if n, _ := s.redis.Get(ctx, failKey).Int(); n >= passwordFailLimit {
			return errRateLimited
		}
	}

	var username, hash string
	err := s.pool.QueryRow(ctx, `
		select username, password_hash from admin_user
		where id = $1 and status = 'active'`, adminID).Scan(&username, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil {
		if s.redis != nil {
			s.recordFailure(ctx, failKey)
		}
		return errInvalidCredentials
	}

	if msg := validateNewPassword(next, current, username); msg != "" {
		return &apiError{Status: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR", Message: msg}
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if _, err := s.pool.Exec(ctx,
		`update admin_user set password_hash = $2 where id = $1`, adminID, string(newHash)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	if s.redis != nil {
		_ = s.redis.Incr(ctx, tokenEpochKey(adminID)).Err()
		_ = s.redis.Del(ctx, failKey, "rl:admin:login:"+username).Err()
	}
	s.logger.Info("admin password changed", "admin_id", adminID, "username", username)
	return nil
}

// validateNewPassword 返回不合规原因；空字符串表示通过。
func validateNewPassword(next, current, username string) string {
	switch {
	case utf8.RuneCountInString(next) < 12:
		return "新密码至少 12 个字符"
	case len(next) > 72:
		return "新密码不能超过 72 字节"
	case next == current:
		return "新密码不能与当前密码相同"
	case strings.EqualFold(strings.TrimSpace(next), username):
		return "新密码不能与用户名相同"
	}
	return ""
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

func (s *Service) sign(rec adminRecord, roles, permissions []string, expiresAt time.Time, epoch int64) (string, error) {
	now := time.Now()
	payload := claims{
		Type:     "admin",
		Username: rec.Username,
		Roles:    roles,
		Perms:    permissions,
		Epoch:    epoch,
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
		Epoch:       payload.Epoch,
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
			principal, err := s.authenticate(r.Context(), token)
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
		principal, err := s.authenticate(r.Context(), cookie.Value)
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

// authenticate 解析令牌并校验令牌版本（改密后会递增，旧令牌立即失效）。
func (s *Service) authenticate(ctx context.Context, raw string) (Principal, error) {
	principal, err := s.parse(raw)
	if err != nil {
		return Principal{}, err
	}
	if !s.tokenFresh(ctx, principal) {
		return Principal{}, errInvalidToken
	}
	return principal, nil
}

// tokenEpochKey 是管理员令牌版本的 Redis 键。
func tokenEpochKey(adminID string) string { return "admin:epoch:" + adminID }

// tokenEpoch 读取当前令牌版本；Redis 不可用或未配置时按 0 处理。
func (s *Service) tokenEpoch(ctx context.Context, adminID string) int64 {
	if s.redis == nil {
		return 0
	}
	v, err := s.redis.Get(ctx, tokenEpochKey(adminID)).Int64()
	if err != nil {
		return 0
	}
	return v
}

// tokenFresh 判定令牌版本是否仍然有效。
// Redis 读失败时放行（避免缓存抖动把全部管理会话打成 401），版本不匹配一律拒绝。
func (s *Service) tokenFresh(ctx context.Context, principal Principal) bool {
	if s.redis == nil {
		return true
	}
	current, err := s.redis.Get(ctx, tokenEpochKey(principal.ID)).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		s.logger.Warn("read admin token epoch", "admin_id", principal.ID, "error", err)
		return true
	}
	return principal.Epoch == current
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
		r.Post("/admin/auth/password", s.handleChangePassword)
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

// handleChangePassword 管理员自助改密；成功后清除 Cookie，要求重新登录。
func (s *Service) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	principal, ok := From(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "未登录")
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(w, r, &req); err != nil ||
		strings.TrimSpace(req.CurrentPassword) == "" || req.NewPassword == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "当前密码与新密码均不能为空")
		return
	}
	if err := s.ChangePassword(r.Context(), principal.ID, req.CurrentPassword, req.NewPassword); err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) {
			writeError(w, apiErr.Status, apiErr.Code, apiErr.Message)
			return
		}
		s.logger.Error("admin change password", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "服务端错误")
		return
	}
	s.clearAdminCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "re_login_required": true})
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
	errInvalidToken       = &apiError{Status: http.StatusUnauthorized, Code: "AUTH_INVALID_TOKEN", Message: "管理员令牌无效或已过期"}
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
