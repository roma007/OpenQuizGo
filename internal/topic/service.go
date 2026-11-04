package topic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/admin"
)

type Pack struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Type          string     `json:"type"`
	SubjectScope  []string   `json:"subject_scope"`
	StageScope    []string   `json:"stage_scope"`
	StartAt       *time.Time `json:"start_at,omitempty"`
	EndAt         *time.Time `json:"end_at,omitempty"`
	IsPaid        bool       `json:"is_paid"`
	Status        string     `json:"status"`
	QuestionCount int        `json:"question_count"`
	QuestionIDs   []string   `json:"question_ids,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Input struct {
	Name         string     `json:"name"`
	Type         string     `json:"type"`
	SubjectScope []string   `json:"subject_scope"`
	StageScope   []string   `json:"stage_scope"`
	StartAt      *time.Time `json:"start_at"`
	EndAt        *time.Time `json:"end_at"`
	IsPaid       bool       `json:"is_paid"`
	Status       string     `json:"status"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) Routes(r chi.Router, guard *admin.Service) {
	r.Group(func(r chi.Router) {
		r.Use(guard.Auth)

		r.With(guard.Require("question.read")).Get("/admin/topics", s.handleList)
		r.With(guard.Require("question.read")).Get("/admin/topics/{id}", s.handleGet)
		r.With(guard.Require("question.write")).Post("/admin/topics", s.handleCreate)
		r.With(guard.Require("question.write")).Put("/admin/topics/{id}", s.handleUpdate)
		r.With(guard.Require("question.write")).Delete("/admin/topics/{id}", s.handleDelete)
		r.With(guard.Require("question.write")).Put("/admin/topics/{id}/questions", s.handleSetQuestions)
	})
}

func (s *Service) List(ctx context.Context) ([]Pack, error) {
	rows, err := s.pool.Query(ctx, `
		select tp.id, tp.name, tp.type, tp.subject_scope::text, tp.stage_scope::text,
		       tp.start_at, tp.end_at, tp.is_paid, tp.status,
		       (select count(*) from topic_question tq where tq.topic_id = tp.id)::int,
		       tp.created_at, tp.updated_at
		from topic_pack tp order by tp.created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	packs := make([]Pack, 0)
	for rows.Next() {
		var p Pack
		var subjects, stages string
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &subjects, &stages,
			&p.StartAt, &p.EndAt, &p.IsPaid, &p.Status, &p.QuestionCount,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(subjects), &p.SubjectScope)
		_ = json.Unmarshal([]byte(stages), &p.StageScope)
		packs = append(packs, p)
	}
	return packs, rows.Err()
}

func (s *Service) Get(ctx context.Context, id string) (Pack, error) {
	var p Pack
	var subjects, stages string
	var ids []string
	err := s.pool.QueryRow(ctx, `
		select tp.id, tp.name, tp.type, tp.subject_scope::text, tp.stage_scope::text,
		       tp.start_at, tp.end_at, tp.is_paid, tp.status,
		       (select count(*) from topic_question tq where tq.topic_id = tp.id)::int,
		       tp.created_at, tp.updated_at,
		       coalesce((select array_agg(question_id::text order by position)
		                 from topic_question where topic_id = tp.id), '{}')
		from topic_pack tp where tp.id = $1`, id).
		Scan(&p.ID, &p.Name, &p.Type, &subjects, &stages, &p.StartAt, &p.EndAt,
			&p.IsPaid, &p.Status, &p.QuestionCount, &p.CreatedAt, &p.UpdatedAt, &ids)
	if errors.Is(err, pgx.ErrNoRows) {
		return Pack{}, apiError{404, "TOPIC_NOT_FOUND", "专题不存在"}
	}
	if err != nil {
		return Pack{}, err
	}
	_ = json.Unmarshal([]byte(subjects), &p.SubjectScope)
	_ = json.Unmarshal([]byte(stages), &p.StageScope)
	p.QuestionIDs = ids
	return p, nil
}

func (s *Service) Create(ctx context.Context, in Input) (Pack, error) {
	if err := validate(in); err != nil {
		return Pack{}, err
	}
	subjects, _ := json.Marshal(in.SubjectScope)
	stages, _ := json.Marshal(in.StageScope)
	var id string
	err := s.pool.QueryRow(ctx, `
		insert into topic_pack (name, type, subject_scope, stage_scope, start_at, end_at, is_paid, status)
		values ($1, $2, $3::jsonb, $4::jsonb, $5, $6, $7, $8) returning id`,
		in.Name, in.Type, string(subjects), string(stages), in.StartAt, in.EndAt, in.IsPaid, in.Status).Scan(&id)
	if err != nil {
		return Pack{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id string, in Input) (Pack, error) {
	if err := validate(in); err != nil {
		return Pack{}, err
	}
	subjects, _ := json.Marshal(in.SubjectScope)
	stages, _ := json.Marshal(in.StageScope)
	tag, err := s.pool.Exec(ctx, `
		update topic_pack set name = $2, type = $3, subject_scope = $4::jsonb,
		       stage_scope = $5::jsonb, start_at = $6, end_at = $7, is_paid = $8,
		       status = $9, updated_at = now()
		where id = $1`,
		id, in.Name, in.Type, string(subjects), string(stages), in.StartAt, in.EndAt, in.IsPaid, in.Status)
	if err != nil {
		return Pack{}, err
	}
	if tag.RowsAffected() == 0 {
		return Pack{}, apiError{404, "TOPIC_NOT_FOUND", "专题不存在"}
	}
	return s.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from topic_pack where id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apiError{404, "TOPIC_NOT_FOUND", "专题不存在"}
	}
	return nil
}

func (s *Service) SetQuestions(ctx context.Context, id string, questionIDs []string) (Pack, error) {
	if len(questionIDs) > 500 {
		return Pack{}, apiError{422, "VALIDATION_ERROR", "单个专题最多 500 题"}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Pack{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from topic_pack where id = $1)`, id).Scan(&exists); err != nil {
		return Pack{}, err
	}
	if !exists {
		return Pack{}, apiError{404, "TOPIC_NOT_FOUND", "专题不存在"}
	}
	if _, err := tx.Exec(ctx, `delete from topic_question where topic_id = $1`, id); err != nil {
		return Pack{}, err
	}
	seen := map[string]bool{}
	position := 0
	for _, qid := range questionIDs {
		if qid == "" || seen[qid] {
			continue
		}
		seen[qid] = true
		if _, err := tx.Exec(ctx, `
			insert into topic_question (topic_id, question_id, position)
			values ($1, $2, $3)`, id, qid, position); err != nil {
			return Pack{}, apiError{422, "QUESTION_INVALID", "题目不存在或已被删除"}
		}
		position++
	}
	if _, err := tx.Exec(ctx, `update topic_pack set updated_at = now() where id = $1`, id); err != nil {
		return Pack{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Pack{}, err
	}
	return s.Get(ctx, id)
}

func validate(in Input) error {
	if in.Name == "" || len(in.Name) > 100 {
		return apiError{422, "VALIDATION_ERROR", "名称需为 1–100 字"}
	}
	types := map[string]bool{"holiday": true, "ip": true, "vacation": true, "daily": true}
	if !types[in.Type] {
		return apiError{422, "VALIDATION_ERROR", "专题类型不合法"}
	}
	statuses := map[string]bool{"draft": true, "online": true, "offline": true}
	if !statuses[in.Status] {
		return apiError{422, "VALIDATION_ERROR", "状态不合法"}
	}
	if in.StartAt != nil && in.EndAt != nil && !in.EndAt.After(*in.StartAt) {
		return apiError{422, "VALIDATION_ERROR", "结束时间需晚于开始时间"}
	}
	return nil
}

type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e apiError) Error() string { return e.Message }

func writeError(w http.ResponseWriter, err error) {
	var e apiError
	if errors.As(err, &e) {
		writeJSON(w, e.Status, map[string]string{"code": e.Code, "message": e.Message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "INTERNAL", "message": "服务端错误"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	packs, err := s.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": packs})
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	pack, err := s.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pack)
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	var in Input
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	pack, err := s.Create(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, pack)
}

func (s *Service) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var in Input
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	pack, err := s.Update(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pack)
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleSetQuestions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		QuestionIDs []string `json:"question_ids"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	pack, err := s.SetQuestions(r.Context(), chi.URLParam(r, "id"), req.QuestionIDs)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pack)
}
