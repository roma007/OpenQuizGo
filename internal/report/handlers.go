package report

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/roma007/OpenQuizGo/internal/identity"
)

// ParentRoutes 注册家长端报告接口。
func (s *Service) ParentRoutes(r chi.Router) {
	r.Get("/family/children/{childID}/reports", s.handleList)
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "week"
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	reports, err := s.List(r.Context(), identity.AccountIDFrom(r.Context()),
		chi.URLParam(r, "childID"), period, limit)
	if err != nil {
		writeReportError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": reports})
}

func writeReportError(w http.ResponseWriter, r *http.Request, err error) {
	var reportErr *Error
	if errors.As(err, &reportErr) {
		writeJSON(w, reportErr.Status, map[string]string{
			"code":       reportErr.Code,
			"message":    reportErr.Message,
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
