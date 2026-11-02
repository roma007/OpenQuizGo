package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorBody 是统一的错误响应结构。
type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// WriteJSON 输出 JSON 响应。
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().Error("encode response", "error", err)
	}
}

// WriteError 输出统一错误结构。
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	WriteJSON(w, status, ErrorBody{
		Code:      code,
		Message:   message,
		RequestID: RequestIDFrom(r.Context()),
	})
}
