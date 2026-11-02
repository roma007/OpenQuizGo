package question

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/roma007/OpenQuizGo/internal/admin"
)

// Routes 注册题库治理路由（均需管理员鉴权）。
func (s *Service) Routes(r chi.Router, guard *admin.Service) {
	r.Group(func(r chi.Router) {
		r.Use(guard.Auth)

		r.With(guard.Require("question.read")).Get("/admin/questions", s.handleList)
		r.With(guard.Require("question.write")).Post("/admin/questions", s.handleCreate)
		r.With(guard.Require("question.import")).Post("/admin/questions/import", s.handleImport)
		r.With(guard.Require("question.read")).Get("/admin/questions/{id}", s.handleGet)
		r.With(guard.Require("question.write")).Put("/admin/questions/{id}", s.handleUpdate)
		r.With(guard.Require("question.write")).Delete("/admin/questions/{id}", s.handleDelete)

		r.With(guard.Require("question.write")).Post("/admin/questions/{id}/submit-review", s.handleAction("submit_review"))
		r.With(guard.Require("question.review")).Post("/admin/questions/{id}/review", s.handleReview)
		r.With(guard.Require("question.publish")).Post("/admin/questions/{id}/publish", s.handleAction("publish"))
		r.With(guard.Require("question.publish")).Post("/admin/questions/{id}/offline", s.handleAction("offline"))
		r.With(guard.Require("question.write")).Post("/admin/questions/{id}/resubmit", s.handleAction("resubmit"))

		r.With(guard.Require("audit.read")).Get("/admin/audit", s.handleAudit)
	})
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	difficulty, _ := strconv.Atoi(q.Get("difficulty"))
	result, err := s.List(r.Context(), Filter{
		Subject:    q.Get("subject"),
		Category:   q.Get("category"),
		Stage:      q.Get("stage"),
		Type:       q.Get("type"),
		Status:     q.Get("status"),
		Keyword:    q.Get("q"),
		TopicID:    q.Get("topic_id"),
		Difficulty: difficulty,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	var in Input
	if err := decodeJSON(w, r, &in); err != nil {
		writeServiceError(w, r, newAPIError(422, "VALIDATION_ERROR", "请求体不是合法 JSON"))
		return
	}
	principal, _ := admin.From(r.Context())
	created, err := s.Create(r.Context(), in, principal, clientIP(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	q, err := s.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (s *Service) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var in Input
	if err := decodeJSON(w, r, &in); err != nil {
		writeServiceError(w, r, newAPIError(422, "VALIDATION_ERROR", "请求体不是合法 JSON"))
		return
	}
	principal, _ := admin.From(r.Context())
	updated, err := s.Update(r.Context(), chi.URLParam(r, "id"), in, principal, clientIP(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) {
	principal, _ := admin.From(r.Context())
	if err := s.Delete(r.Context(), chi.URLParam(r, "id"), principal, clientIP(r)); err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, _ := admin.From(r.Context())
		q, err := s.Transition(r.Context(), chi.URLParam(r, "id"), action, principal, clientIP(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, q)
	}
}

func (s *Service) handleReview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Approve bool `json:"approve"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, newAPIError(422, "VALIDATION_ERROR", "请求体不是合法 JSON"))
		return
	}
	action := "reject"
	if req.Approve {
		action = "approve"
	}
	principal, _ := admin.From(r.Context())
	q, err := s.Transition(r.Context(), chi.URLParam(r, "id"), action, principal, clientIP(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (s *Service) handleImport(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
	if err != nil {
		writeServiceError(w, r, newAPIError(413, "PAYLOAD_TOO_LARGE", "导入文件不能超过 10MB"))
		return
	}

	var inputs []Input
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	switch {
	case strings.Contains(contentType, "text/csv"):
		parsed, problems := parseCSV(body)
		if len(problems) > 0 {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"code":    "IMPORT_VALIDATION_FAILED",
				"message": "CSV 解析失败",
				"errors":  problems,
			})
			return
		}
		inputs = parsed
	default:
		var wrapper struct {
			Questions []Input `json:"questions"`
		}
		if err := json.Unmarshal(body, &wrapper); err == nil && len(wrapper.Questions) > 0 {
			inputs = wrapper.Questions
		} else if err := json.Unmarshal(body, &inputs); err != nil {
			writeServiceError(w, r, newAPIError(422, "VALIDATION_ERROR", "JSON 需为数组或 {questions:[...]}"))
			return
		}
	}

	principal, _ := admin.From(r.Context())
	result, err := s.Import(r.Context(), inputs, principal, clientIP(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	if len(result.Errors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"code":    "IMPORT_VALIDATION_FAILED",
			"message": "存在校验失败的题目，整批未导入",
			"errors":  result.Errors,
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleAudit(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := s.Audit(r.Context(), r.URL.Query().Get("target_id"), limit)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

/* ------------------------------ CSV ------------------------------ */

var csvColumns = []string{
	"subject", "category", "stage", "grade_tag", "difficulty", "type", "stem",
	"stem_media", "options", "answer", "answer_alias", "explain", "duration_limit", "topic_id", "schema_ver",
}

func parseCSV(body []byte) ([]Input, []ImportError) {
	reader := csv.NewReader(bytes.NewReader(body))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, []ImportError{{Row: 0, Field: "csv", Message: err.Error()}}
	}
	if len(rows) < 2 {
		return nil, []ImportError{{Row: 0, Field: "csv", Message: "CSV 至少需要表头和数据行"}}
	}

	index := map[string]int{}
	for i, name := range rows[0] {
		index[strings.TrimSpace(strings.ToLower(name))] = i
	}
	for _, required := range []string{"subject", "category", "stage", "difficulty", "type", "stem", "answer"} {
		if _, ok := index[required]; !ok {
			return nil, []ImportError{{Row: 1, Field: "csv", Message: "缺少列：" + required}}
		}
	}

	inputs := make([]Input, 0, len(rows)-1)
	problems := make([]ImportError, 0)
	for rowIndex, row := range rows[1:] {
		if isEmptyRow(row) {
			continue
		}
		input := Input{}
		parseErr := false
		cell := func(name string) string {
			i, ok := index[name]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		input.Subject = cell("subject")
		input.Category = cell("category")
		input.Stage = cell("stage")
		input.GradeTag = cell("grade_tag")
		input.Type = cell("type")
		input.Stem = cell("stem")
		input.Explain = cell("explain")
		input.StemMedia = csvJSON(cell("stem_media"))
		input.Options = csvJSON(cell("options"))
		input.Answer = csvJSON(cell("answer"))
		input.AnswerAlias = csvJSON(cell("answer_alias"))
		if value := cell("difficulty"); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil {
				problems = append(problems, ImportError{Row: rowIndex + 2, Field: "difficulty", Message: "必须是整数"})
				parseErr = true
			}
			input.Difficulty = n
		}
		if value := cell("duration_limit"); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil {
				problems = append(problems, ImportError{Row: rowIndex + 2, Field: "duration_limit", Message: "必须是整数"})
				parseErr = true
			}
			input.DurationLimit = &n
		}
		if value := cell("topic_id"); value != "" {
			topic := value
			input.TopicID = &topic
		}
		if value := cell("schema_ver"); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil {
				problems = append(problems, ImportError{Row: rowIndex + 2, Field: "schema_ver", Message: "必须是整数"})
				parseErr = true
			}
			input.SchemaVer = n
		}
		if parseErr {
			continue
		}
		inputs = append(inputs, input)
	}
	return inputs, problems
}

func csvJSON(value string) json.RawMessage {
	if value == "" {
		return nil
	}
	return json.RawMessage(value)
}

func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

/* ------------------------------ 响应 ------------------------------ */

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		writeJSON(w, apiErr.Status, map[string]string{
			"code":       apiErr.Code,
			"message":    apiErr.Message,
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

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	return json.NewDecoder(r.Body).Decode(dst)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
