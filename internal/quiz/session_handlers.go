package quiz

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/roma007/OpenQuizGo/internal/identity"
)

// DeviceRoutes 注册电视端答题路由（需设备令牌）。
func (s *Service) DeviceRoutes(r chi.Router) {
	r.Post("/quiz/sessions", s.handleCreateSession)
	r.Get("/quiz/sessions/{sessionID}", s.handleGetSession)
	r.Post("/quiz/sessions/{sessionID}/answers", s.handleSubmitAnswer)
	r.Post("/quiz/sessions/{sessionID}/finish", s.handleFinishSession)
}

func (s *Service) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := identity.DeviceFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "设备未授权")
		return
	}
	result, err := s.CreateSession(r.Context(), principal)
	if err != nil {
		writeQuizError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleGetSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := identity.DeviceFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "设备未授权")
		return
	}
	result, err := s.GetSession(r.Context(), principal, chi.URLParam(r, "sessionID"))
	if err != nil {
		writeQuizError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleSubmitAnswer(w http.ResponseWriter, r *http.Request) {
	principal, ok := identity.DeviceFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "设备未授权")
		return
	}
	var submission AnswerSubmission
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&submission); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求体不是合法 JSON")
		return
	}
	if submission.Token == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "缺少题令牌")
		return
	}
	result, err := s.SubmitAnswer(r.Context(), principal, chi.URLParam(r, "sessionID"), submission)
	if err != nil {
		writeQuizError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleFinishSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := identity.DeviceFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "设备未授权")
		return
	}
	result, err := s.FinishSession(r.Context(), principal, chi.URLParam(r, "sessionID"))
	if err != nil {
		writeQuizError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeQuizError(w http.ResponseWriter, r *http.Request, err error) {
	var quizErr *Error
	if errors.As(err, &quizErr) {
		body := map[string]any{
			"code":       quizErr.Code,
			"message":    quizErr.Message,
			"request_id": w.Header().Get("X-Request-Id"),
		}
		if len(quizErr.Warnings) > 0 {
			body["warnings"] = quizErr.Warnings
		}
		if quizErr.SessionID != "" {
			body["session_id"] = quizErr.SessionID
		}
		writeJSON(w, quizErr.Status, body)
		return
	}
	writeError(w, http.StatusInternalServerError, "INTERNAL", "服务端错误")
}
