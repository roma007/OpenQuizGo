package daily

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/identity"
)

type QuestionView struct {
	ID      string          `json:"id"`
	Stem    string          `json:"stem"`
	Type    string          `json:"type"`
	Options json.RawMessage `json:"options,omitempty"`
}

type DailyState struct {
	Date          string          `json:"date"`
	Question      *QuestionView   `json:"question,omitempty"`
	Answered      bool            `json:"answered"`
	Correct       *bool           `json:"correct,omitempty"`
	CorrectAnswer json.RawMessage `json:"correct_answer,omitempty"`
	Explain       string          `json:"explain,omitempty"`
	Points        int             `json:"points"`
	Streak        int             `json:"streak"`
	TotalPoints   int             `json:"total_points"`
}

type Service struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{pool: pool, logger: logger}
}

func (s *Service) Routes(r chi.Router) {
	r.Get("/family/children/{childID}/daily", s.handleDaily)
	r.Post("/family/children/{childID}/daily/answers", s.handleAnswer)
}

func (s *Service) handleDaily(w http.ResponseWriter, r *http.Request) {
	childID := chi.URLParam(r, "childID")
	if err := s.ownsChild(r.Context(), identity.AccountIDFrom(r.Context()), childID); err != nil {
		writeError(w, r, err)
		return
	}
	state, err := s.State(r.Context(), childID)
	if err != nil {
		s.logger.Error("daily state", "error", err, "child_id", childID)
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Service) handleAnswer(w http.ResponseWriter, r *http.Request) {
	childID := chi.URLParam(r, "childID")
	if err := s.ownsChild(r.Context(), identity.AccountIDFrom(r.Context()), childID); err != nil {
		writeError(w, r, err)
		return
	}
	var req struct {
		Answer json.RawMessage `json:"answer"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, r, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	state, err := s.Answer(r.Context(), childID, req.Answer)
	if err != nil {
		s.logger.Error("daily answer", "error", err, "child_id", childID)
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e apiError) Error() string { return e.Message }

func (s *Service) ownsChild(ctx context.Context, accountID, childID string) error {
	if accountID == "" {
		return apiError{401, "AUTH_REQUIRED", "家长未登录"}
	}
	var exists bool
	err := s.pool.QueryRow(ctx, `
		select exists (
			select 1 from child_profile cp
			join family_member fm on fm.family_id = cp.family_id
			where cp.id = $1 and fm.account_id = $2
			  and cp.family_id = coalesce(
			      (select active_family_id from parent_account where id = $2), fm.family_id))`,
		childID, accountID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return apiError{404, "CHILD_NOT_FOUND", "孩子档案不存在"}
	}
	return nil
}

func (s *Service) today(ctx context.Context, childID string) (time.Time, *time.Location, error) {
	tz := "Asia/Shanghai"
	_ = s.pool.QueryRow(ctx, `select timezone from watch_rule where child_id = $1`, childID).Scan(&tz)
	location, err := time.LoadLocation(tz)
	if err != nil {
		location = time.UTC
	}
	now := time.Now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	return start, location, nil
}

type questionRecord struct {
	ID      string
	Stem    string
	Type    string
	Options json.RawMessage
	Answer  json.RawMessage
	Explain string
}

func (s *Service) pickQuestion(ctx context.Context, childID string, day time.Time) (questionRecord, error) {
	var q questionRecord
	err := s.pool.QueryRow(ctx, `
		select id, stem, type_code, coalesce(options::text, '[]'), answer::text, coalesce(explain, '')
		from question
		where status = 'online' and type_code in ('choice', 'image') and options is not null
		order by md5(id::text || $1::text)
		limit 1`, day.Format("2006-01-02")).
		Scan(&q.ID, &q.Stem, &q.Type, &q.Options, &q.Answer, &q.Explain)
	if errors.Is(err, pgx.ErrNoRows) {
		return questionRecord{}, apiError{409, "QUESTION_POOL_EMPTY", "题库暂无可用题目"}
	}
	if err != nil {
		return questionRecord{}, fmt.Errorf("pick question: %w", err)
	}
	return q, nil
}

func (s *Service) State(ctx context.Context, childID string) (DailyState, error) {
	day, _, err := s.today(ctx, childID)
	if err != nil {
		return DailyState{}, err
	}
	question, err := s.pickQuestion(ctx, childID, day)
	if err != nil {
		return DailyState{}, err
	}
	state := DailyState{
		Date:     day.Format("2006-01-02"),
		Question: &QuestionView{ID: question.ID, Stem: question.Stem, Type: question.Type, Options: question.Options},
	}
	var correct bool
	var points int
	err = s.pool.QueryRow(ctx, `
		select correct, points from child_daily_answer
		where child_id = $1 and answer_date = $2`, childID, day).Scan(&correct, &points)
	if err == nil {
		state.Answered = true
		state.Correct = &correct
		state.Points = points
		state.CorrectAnswer = question.Answer
		state.Explain = question.Explain
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return DailyState{}, err
	}
	if err := s.stats(ctx, childID, day, &state); err != nil {
		return DailyState{}, err
	}
	return state, nil
}

func (s *Service) Answer(ctx context.Context, childID string, answer json.RawMessage) (DailyState, error) {
	day, _, err := s.today(ctx, childID)
	if err != nil {
		return DailyState{}, err
	}
	question, err := s.pickQuestion(ctx, childID, day)
	if err != nil {
		return DailyState{}, err
	}
	correct := equalSet(question.Answer, answer)
	points := 0
	if correct {
		points = 10
	}
	_, err = s.pool.Exec(ctx, `
		insert into child_daily_answer (child_id, answer_date, question_id, correct, points)
		values ($1, $2, $3, $4, $5)
		on conflict (child_id, answer_date) do nothing`, childID, day, question.ID, correct, points)
	if err != nil {
		return DailyState{}, err
	}
	return s.State(ctx, childID)
}

func (s *Service) stats(ctx context.Context, childID string, day time.Time, state *DailyState) error {
	if err := s.pool.QueryRow(ctx,
		`select coalesce(sum(points), 0)::int from child_daily_answer where child_id = $1`,
		childID).Scan(&state.TotalPoints); err != nil {
		return err
	}
	streak, err := s.streak(ctx, childID, day)
	if err != nil {
		return err
	}
	state.Streak = streak
	return nil
}

func (s *Service) streak(ctx context.Context, childID string, day time.Time) (int, error) {
	rows, err := s.pool.Query(ctx, `
		select answer_date from child_daily_answer
		where child_id = $1 order by answer_date desc limit 365`, childID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	dates := make([]time.Time, 0)
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return 0, err
		}
		dates = append(dates, d)
	}
	if len(dates) == 0 {
		return 0, nil
	}
	// 若今天未答，从昨天开始计算；按日期字符串比较避免时区差异。
	cursor := day
	if dateKey(dates[0]) != dateKey(day) {
		cursor = day.AddDate(0, 0, -1)
	}
	count := 0
	for _, d := range dates {
		if dateKey(d) == dateKey(cursor) {
			count++
			cursor = cursor.AddDate(0, 0, -1)
			continue
		}
		if dateKey(d) < dateKey(cursor) {
			break
		}
	}
	return count, nil
}

func dateKey(t time.Time) string {
	return t.Format("2006-01-02")
}

func equalSet(expected, given json.RawMessage) bool {
	want, ok1 := stringList(expected)
	got, ok2 := stringList(given)
	if !ok1 || !ok2 || len(want) != len(got) {
		return false
	}
	sort.Strings(want)
	sort.Strings(got)
	for i := range want {
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

func stringList(raw json.RawMessage) ([]string, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, true
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, true
	}
	return nil, false
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
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
