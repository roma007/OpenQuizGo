package quiz

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/roma007/OpenQuizGo/internal/admin"
)

// Routes 注册组卷相关管理端接口。
func (s *Service) Routes(r chi.Router, guard *admin.Service) {
	r.Group(func(r chi.Router) {
		r.Use(guard.Auth)
		r.With(guard.Require("question.read")).Post("/admin/quiz/preview", s.handlePreview)
	})
}

func (s *Service) handlePreview(w http.ResponseWriter, r *http.Request) {
	var req SelectRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求体不是合法 JSON")
		return
	}
	result, err := s.Select(r.Context(), req)
	if err != nil {
		var quizErr *Error
		if errors.As(err, &quizErr) {
			body := map[string]any{
				"code":    quizErr.Code,
				"message": quizErr.Message,
			}
			if len(quizErr.Warnings) > 0 {
				body["warnings"] = quizErr.Warnings
			}
			writeJSON(w, quizErr.Status, body)
			return
		}
		s.logger.Error("select questions", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "服务端错误")
		return
	}
	writeJSON(w, http.StatusOK, PreviewResponse{Result: result, GeneratedAt: time.Now()})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
