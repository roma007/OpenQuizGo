package rule

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/roma007/OpenQuizGo/internal/identity"
)

// ParentRoutes 注册家长端规则接口（需家长令牌）。
func (s *Service) ParentRoutes(r chi.Router) {
	r.Get("/family/children/{childID}/rule", s.handleGet)
	r.Put("/family/children/{childID}/rule", s.handleUpdate)
	r.Post("/family/children/{childID}/rule/preview", s.handlePreview)
}

// DeviceRoutes 注册电视端上下文接口（需设备令牌）。
func (s *Service) DeviceRoutes(r chi.Router) {
	r.Get("/child/context", s.handleContext)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	rule, version, err := s.Get(r.Context(), identity.AccountIDFrom(r.Context()), chi.URLParam(r, "childID"))
	if err != nil {
		writeRuleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rule": rule, "version": version})
}

func (s *Service) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PIN  string `json:"pin"`
		Rule Rule   `json:"rule"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRuleError(w, r, newError(422, "VALIDATION_ERROR", "请求体不是合法 JSON"))
		return
	}
	updated, version, err := s.Update(r.Context(), identity.AccountIDFrom(r.Context()),
		chi.URLParam(r, "childID"), req.PIN, req.Rule, clientIP(r))
	if err != nil {
		writeRuleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rule": updated, "version": version})
}

func (s *Service) handlePreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rule Rule `json:"rule"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRuleError(w, r, newError(422, "VALIDATION_ERROR", "请求体不是合法 JSON"))
		return
	}
	result, err := s.Preview(r.Context(), identity.AccountIDFrom(r.Context()),
		chi.URLParam(r, "childID"), req.Rule)
	if err != nil {
		writeRuleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleContext(w http.ResponseWriter, r *http.Request) {
	principal, ok := identity.DeviceFrom(r.Context())
	if !ok {
		writeRuleError(w, r, newError(401, "AUTH_INVALID_TOKEN", "设备未授权"))
		return
	}
	result, err := s.Context(r.Context(), principal.ChildID)
	if err != nil {
		writeRuleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeRuleError(w http.ResponseWriter, r *http.Request, err error) {
	var ruleErr *Error
	if errors.As(err, &ruleErr) {
		writeJSON(w, ruleErr.Status, map[string]string{
			"code":       ruleErr.Code,
			"message":    ruleErr.Message,
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

func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			if i > 0 && host[0] == '[' {
				return host[1 : i-1]
			}
			return host[:i]
		}
	}
	return host
}
