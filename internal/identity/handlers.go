package identity

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type contextKey int

const (
	accountIDKey contextKey = iota
	deviceKey
)

// AccountIDFrom 返回家长账号 ID。
func AccountIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(accountIDKey).(string)
	return v
}

// DeviceFrom 返回设备主体。
func DeviceFrom(ctx context.Context) (DevicePrincipal, bool) {
	v, ok := ctx.Value(deviceKey).(DevicePrincipal)
	return v, ok
}

// ParentAuth 校验家长访问令牌。
func (s *Service) ParentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, r, ErrInvalidToken)
			return
		}
		accountID, err := s.tokens.parseAccess(token)
		if err != nil {
			writeError(w, r, ErrInvalidToken)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), accountIDKey, accountID)))
	})
}

// DeviceAuth 校验设备令牌。
func (s *Service) DeviceAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, r, ErrInvalidToken)
			return
		}
		principal, err := s.repo.deviceByToken(r.Context(), hashToken(token))
		if err != nil {
			writeError(w, r, newError("AUTH_DEVICE_UNBOUND", http.StatusForbidden, "设备未绑定或令牌已失效"))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), deviceKey, principal)))
	})
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

/* ------------------------------ 响应 ------------------------------ */

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var e *Error
	if errors.As(err, &e) {
		writeJSON(w, e.Status, errorBody{
			Code:      e.Code,
			Message:   e.Message,
			RequestID: w.Header().Get("X-Request-Id"),
		})
		return
	}
	slog.Default().Error("unhandled identity error", "error", err, "path", r.URL.Path)
	writeJSON(w, http.StatusInternalServerError, errorBody{
		Code:    "INTERNAL",
		Message: "服务端错误",
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		return newError("VALIDATION_ERROR", http.StatusUnprocessableEntity, "请求体不是合法 JSON")
	}
	return nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

/* ------------------------------ 路由 ------------------------------ */

// Routes 注册身份相关路由。
func (s *Service) Routes(r chi.Router) {
	r.Post("/auth/sms/code", s.handleSendCode)
	r.Post("/auth/login", s.handleLogin)
	r.Post("/auth/refresh", s.handleRefresh)

	r.Group(func(r chi.Router) {
		r.Use(s.ParentAuth)

		r.Post("/auth/logout", s.handleLogout)
		r.Get("/family/me", s.handleMe)

		r.Get("/family/members", s.handleListMembers)
		r.Post("/family/members/invite", s.handleInvite)
		r.Post("/family/members/join", s.handleJoin)

		r.Get("/family/children", s.handleListChildren)
		r.Post("/family/children", s.handleCreateChild)
		r.Get("/family/children/{childID}", s.handleGetChild)
		r.Put("/family/children/{childID}", s.handleUpdateChild)

		r.Post("/family/pin", s.handleSetPIN)
		r.Post("/family/pin/verify", s.handleVerifyPIN)

		r.Get("/family/devices", s.handleListDevices)
		r.Post("/family/devices/activate", s.handleConfirmActivation)
		r.Delete("/family/devices/{deviceID}", s.handleUnbindDevice)
	})

	r.Post("/device/activate/code", s.handleActivationCode)
	r.Post("/device/token", s.handleDeviceToken)

	r.Group(func(r chi.Router) {
		r.Use(s.DeviceAuth)
		r.Get("/device/me", s.handleDeviceMe)
		r.Delete("/device/me", s.handleUnbindSelf)
	})
}

/* ------------------------------ 处理器 ------------------------------ */

func (s *Service) handleSendCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.SendLoginCode(r.Context(), req.Phone); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"expires_in": int64(smsCodeTTL.Seconds()),
	})
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.Login(r.Context(), req.Phone, req.Code, r.UserAgent(), clientIP(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.Refresh(r.Context(), req.RefreshToken, r.UserAgent(), clientIP(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.Logout(r.Context(), req.RefreshToken); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleMe(w http.ResponseWriter, r *http.Request) {
	account, err := s.Me(r.Context(), AccountIDFrom(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (s *Service) handleListChildren(w http.ResponseWriter, r *http.Request) {
	children, err := s.ListChildren(r.Context(), AccountIDFrom(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, children)
}

func (s *Service) handleCreateChild(w http.ResponseWriter, r *http.Request) {
	var in ChildInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	child, err := s.CreateChild(r.Context(), AccountIDFrom(r.Context()), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, child)
}

func (s *Service) handleGetChild(w http.ResponseWriter, r *http.Request) {
	child, err := s.GetChild(r.Context(), AccountIDFrom(r.Context()), chi.URLParam(r, "childID"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, child)
}

func (s *Service) handleUpdateChild(w http.ResponseWriter, r *http.Request) {
	var in ChildInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	child, err := s.UpdateChild(r.Context(), AccountIDFrom(r.Context()), chi.URLParam(r, "childID"), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, child)
}

func (s *Service) handleSetPIN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPIN string `json:"current_pin"`
		NewPIN     string `json:"new_pin"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.SetPIN(r.Context(), AccountIDFrom(r.Context()), req.CurrentPIN, req.NewPIN); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleVerifyPIN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PIN string `json:"pin"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.VerifyPIN(r.Context(), AccountIDFrom(r.Context()), req.PIN); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.ListDevices(r.Context(), AccountIDFrom(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (s *Service) handleActivationCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
		Platform string `json:"platform"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	code, err := s.CreateActivationCode(r.Context(), req.DeviceID, req.Platform)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, code)
}

func (s *Service) handleConfirmActivation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code    string `json:"code"`
		ChildID string `json:"child_id"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	deviceID, err := s.ConfirmActivation(r.Context(), AccountIDFrom(r.Context()), req.Code, req.ChildID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "confirmed", "device_id": deviceID})
}

func (s *Service) handleDeviceToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code     string `json:"code"`
		DeviceID string `json:"device_id"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	token, pending, err := s.ExchangeDeviceToken(r.Context(), req.Code, req.DeviceID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if pending {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
		return
	}
	writeJSON(w, http.StatusOK, token)
}

func (s *Service) handleUnbindSelf(w http.ResponseWriter, r *http.Request) {
	principal, ok := DeviceFrom(r.Context())
	if !ok {
		writeError(w, r, ErrInvalidToken)
		return
	}
	if err := s.UnbindSelf(r.Context(), principal.DeviceID); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unbound"})
}

func (s *Service) handleUnbindDevice(w http.ResponseWriter, r *http.Request) {
	if err := s.UnbindDevice(r.Context(), AccountIDFrom(r.Context()), chi.URLParam(r, "deviceID")); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleDeviceMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := DeviceFrom(r.Context())
	if !ok {
		writeError(w, r, ErrInvalidToken)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_id":  principal.DeviceID,
		"child_id":   principal.ChildID,
		"child_name": principal.ChildName,
	})
}

func (s *Service) handleInvite(w http.ResponseWriter, r *http.Request) {
	invite, err := s.InviteMember(r.Context(), AccountIDFrom(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, invite)
}

func (s *Service) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	familyID, err := s.JoinFamily(r.Context(), AccountIDFrom(r.Context()), req.Code)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"family_id": familyID})
}

func (s *Service) handleListMembers(w http.ResponseWriter, r *http.Request) {
	members, err := s.ListMembers(r.Context(), AccountIDFrom(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}
