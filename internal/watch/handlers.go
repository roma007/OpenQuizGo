package watch

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/roma007/OpenQuizGo/internal/identity"
)

// DeviceRoutes 注册电视端计量与设备上报接口。
func (s *Service) DeviceRoutes(r chi.Router) {
	r.Post("/watch/grants/{grantID}/heartbeat", s.handleHeartbeat)
	r.Get("/watch/grants/active", s.handleActiveGrant)
	r.Put("/device/capability", s.handleCapability)
	r.Post("/device/tamper", s.handleTamper)
}

func (s *Service) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	principal, ok := identity.DeviceFrom(r.Context())
	if !ok {
		writeWatchError(w, r, newError(401, "AUTH_INVALID_TOKEN", "设备未授权"))
		return
	}
	var req HeartbeatRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeWatchError(w, r, newError(422, "VALIDATION_ERROR", "请求体不是合法 JSON"))
		return
	}
	result, err := s.Heartbeat(r.Context(), principal, chi.URLParam(r, "grantID"), req)
	if err != nil {
		writeWatchError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleActiveGrant(w http.ResponseWriter, r *http.Request) {
	principal, ok := identity.DeviceFrom(r.Context())
	if !ok {
		writeWatchError(w, r, newError(401, "AUTH_INVALID_TOKEN", "设备未授权"))
		return
	}
	grant, err := s.ActiveGrant(r.Context(), principal)
	if err != nil {
		writeWatchError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, grant)
}

func (s *Service) handleCapability(w http.ResponseWriter, r *http.Request) {
	principal, ok := identity.DeviceFrom(r.Context())
	if !ok {
		writeWatchError(w, r, newError(401, "AUTH_INVALID_TOKEN", "设备未授权"))
		return
	}
	var capability Capability
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&capability); err != nil {
		writeWatchError(w, r, newError(422, "VALIDATION_ERROR", "请求体不是合法 JSON"))
		return
	}
	if err := s.ReportCapability(r.Context(), principal, capability); err != nil {
		writeWatchError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleTamper(w http.ResponseWriter, r *http.Request) {
	principal, ok := identity.DeviceFrom(r.Context())
	if !ok {
		writeWatchError(w, r, newError(401, "AUTH_INVALID_TOKEN", "设备未授权"))
		return
	}
	var event TamperEvent
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeWatchError(w, r, newError(422, "VALIDATION_ERROR", "请求体不是合法 JSON"))
		return
	}
	if err := s.ReportTamper(r.Context(), principal, event); err != nil {
		writeWatchError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeWatchError(w http.ResponseWriter, r *http.Request, err error) {
	var watchErr *Error
	if errors.As(err, &watchErr) {
		writeJSON(w, watchErr.Status, map[string]string{
			"code":       watchErr.Code,
			"message":    watchErr.Message,
			"request_id": w.Header().Get("X-Request-Id"),
		})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{
		"code":       "INTERNAL",
		"message":    "服务端错误",
		"request_id": w.Header().Get("X-Request-Id"),
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
