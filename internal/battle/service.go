package battle

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/roma007/OpenQuizGo/internal/identity"
)

const (
	defaultQuestions = 5
	minQuestions     = 3
	maxQuestions     = 10
	roomCodeTTL      = 10 * time.Minute
)

type Service struct {
	pool   *pgxpool.Pool
	events *eventBroker
}

func New(pool *pgxpool.Pool, rdb *redis.Client) *Service {
	return &Service{pool: pool, events: newEventBroker(rdb)}
}

func (s *Service) Routes(r chi.Router) {
	r.Get("/family/battles/ranking", s.handleRanking)
	r.Get("/family/children/{childID}/battles", s.handleHistory)
	r.Post("/family/children/{childID}/battles", s.handleCreate)
	r.Post("/family/children/{childID}/battles/join", s.handleJoin)
	r.Get("/family/children/{childID}/battles/{battleID}", s.handleGet)
	r.Get("/family/children/{childID}/battles/{battleID}/events", s.handleEvents)
	r.Post("/family/children/{childID}/battles/{battleID}/answers", s.handleSubmit)
}

type QuestionView struct {
	ID      string          `json:"id"`
	Stem    string          `json:"stem"`
	Type    string          `json:"type"`
	Options json.RawMessage `json:"options,omitempty"`
}

type AnswerView struct {
	Side          string          `json:"side"`
	QuestionID    string          `json:"question_id"`
	Answer        json.RawMessage `json:"answer"`
	Correct       bool            `json:"correct"`
	CorrectAnswer json.RawMessage `json:"correct_answer,omitempty"`
	Explain       string          `json:"explain,omitempty"`
}

type ResultView struct {
	Winner      string `json:"winner"`
	ChildScore  int    `json:"child_score"`
	ParentScore int    `json:"parent_score"`
}

type State struct {
	ID            string         `json:"id"`
	Status        string         `json:"status"`
	QuestionCount int            `json:"question_count"`
	Questions     []QuestionView `json:"questions"`
	ChildScore    int            `json:"child_score"`
	ParentScore   int            `json:"parent_score"`
	Mode          string         `json:"mode"`
	JoinCode      string         `json:"join_code,omitempty"`
	// JoinCodeExpiresAt 仅未结束的 room 下发。
	JoinCodeExpiresAt *time.Time   `json:"join_code_expires_at,omitempty"`
	Result            *ResultView  `json:"result,omitempty"`
	Answers           []AnswerView `json:"answers,omitempty"`
}

type Summary struct {
	ID            string     `json:"id"`
	Mode          string     `json:"mode"`
	Status        string     `json:"status"`
	QuestionCount int        `json:"question_count"`
	ChildScore    int        `json:"child_score"`
	ParentScore   int        `json:"parent_score"`
	CreatedAt     time.Time  `json:"created_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

type RankingEntry struct {
	ChildID       string   `json:"child_id"`
	ChildName     string   `json:"child_name"`
	Battles       int      `json:"battles"`
	Wins          int      `json:"wins"`
	Draws         int      `json:"draws"`
	Losses        int      `json:"losses"`
	Points        int      `json:"points"`
	CurrentStreak int      `json:"current_streak"`
	BestStreak    int      `json:"best_streak"`
	Medals        []string `json:"medals"`
}

type Submission struct {
	QuestionID string          `json:"question_id"`
	Answer     json.RawMessage `json:"answer"`
}

type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e apiError) Error() string { return e.Message }

type questionRecord struct {
	ID      string
	Stem    string
	Type    string
	Options json.RawMessage
	Answer  json.RawMessage
	Explain string
}

type battleRecord struct {
	ID                string
	FamilyID          string
	Status            string
	Mode              string
	JoinCode          string
	JoinCodeExpiresAt *time.Time
	QuestionIDs       []string
	ChildScore        int
	ParentScore       int
}

const codeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

func randomCode() string {
	code := make([]byte, 6)
	for i := range code {
		code[i] = codeAlphabet[rand.Intn(len(codeAlphabet))]
	}
	return string(code)
}

func (s *Service) handleRanking(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "all"
	}
	if period != "all" && period != "week" && period != "month" {
		writeError(w, apiError{422, "VALIDATION_ERROR", "period 仅支持 all/week/month"})
		return
	}
	items, err := s.Ranking(r.Context(), identity.AccountIDFrom(r.Context()), period)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) handleHistory(w http.ResponseWriter, r *http.Request) {
	childID := chi.URLParam(r, "childID")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.History(r.Context(), identity.AccountIDFrom(r.Context()), childID, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) {
	childID := chi.URLParam(r, "childID")
	accountID := identity.AccountIDFrom(r.Context())
	if _, err := s.ownsChild(r.Context(), accountID, childID); err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		QuestionCount int    `json:"question_count"`
		Mode          string `json:"mode"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req)
	}
	count := req.QuestionCount
	if count == 0 {
		count = defaultQuestions
	}
	if count < minQuestions || count > maxQuestions {
		writeError(w, apiError{422, "VALIDATION_ERROR", "题数需在 3–10 之间"})
		return
	}
	state, err := s.Create(r.Context(), accountID, childID, count, req.Mode)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, state)
}

func (s *Service) handleJoin(w http.ResponseWriter, r *http.Request) {
	childID := chi.URLParam(r, "childID")
	accountID := identity.AccountIDFrom(r.Context())
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	state, err := s.Join(r.Context(), accountID, childID, req.Code)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Service) handleEvents(w http.ResponseWriter, r *http.Request) {
	childID := chi.URLParam(r, "childID")
	battleID := chi.URLParam(r, "battleID")
	if _, err := s.ownsChild(r.Context(), identity.AccountIDFrom(r.Context()), childID); err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.load(r.Context(), childID, battleID); err != nil {
		writeError(w, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, apiError{500, "STREAM_UNSUPPORTED", "当前连接不支持事件流"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ch, unsubscribe := s.events.subscribe(r.Context(), battleID)
	defer unsubscribe()
	_, _ = w.Write([]byte("event: update\ndata: {}\n\n"))
	flusher.Flush()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			_, _ = w.Write([]byte("event: update\ndata: {}\n\n"))
			flusher.Flush()
		case <-ticker.C:
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		}
	}
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	childID := chi.URLParam(r, "childID")
	if _, err := s.ownsChild(r.Context(), identity.AccountIDFrom(r.Context()), childID); err != nil {
		writeError(w, err)
		return
	}
	state, err := s.Get(r.Context(), childID, chi.URLParam(r, "battleID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Service) handleSubmit(w http.ResponseWriter, r *http.Request) {
	childID := chi.URLParam(r, "childID")
	if _, err := s.ownsChild(r.Context(), identity.AccountIDFrom(r.Context()), childID); err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Side    string       `json:"side"`
		Answers []Submission `json:"answers"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, apiError{422, "VALIDATION_ERROR", "请求体不合法"})
		return
	}
	state, err := s.Submit(r.Context(), childID, chi.URLParam(r, "battleID"), req.Side, req.Answers)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// Ranking 返回家庭内孩子的对战排名。
func (s *Service) Ranking(ctx context.Context, accountID, period string) ([]RankingEntry, error) {
	if period == "" {
		period = "all"
	}
	if accountID == "" {
		return nil, apiError{401, "AUTH_REQUIRED", "家长未登录"}
	}
	var familyID *string
	err := s.pool.QueryRow(ctx, `
		select coalesce(
		    (select active_family_id from parent_account where id = $1),
		    (select family_id from family_member where account_id = $1 limit 1))`,
		accountID).Scan(&familyID)
	if err != nil {
		return nil, err
	}
	if familyID == nil {
		return []RankingEntry{}, nil
	}

	rows, err := s.pool.Query(ctx, `
		select id, nickname from child_profile
		where family_id = $1 order by created_at`, *familyID)
	if err != nil {
		return nil, err
	}
	type childInfo struct {
		name string
	}
	children := map[string]*childInfo{}
	order := make([]string, 0)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		children[id] = &childInfo{name: name}
		order = append(order, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	type aggregate struct {
		battles, wins, draws, losses, points, run, best, current int
	}
	stats := map[string]*aggregate{}
	for id := range children {
		stats[id] = &aggregate{}
	}
	rows, err = s.pool.Query(ctx, `
		select b.child_id, b.child_score, b.parent_score
		from family_battle b
		join child_profile cp on cp.id = b.child_id
		where cp.family_id = $1 and b.status = 'finished'
		  and ($2 = 'all' or b.finished_at >= case $2
		       when 'week' then date_trunc('week', now())
		       when 'month' then date_trunc('month', now())
		       else now() end)
		order by b.child_id, b.finished_at`, *familyID, period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var childID string
		var childScore, parentScore int
		if err := rows.Scan(&childID, &childScore, &parentScore); err != nil {
			return nil, err
		}
		agg := stats[childID]
		if agg == nil {
			continue
		}
		agg.battles++
		switch {
		case childScore > parentScore:
			agg.wins++
			agg.run++
			if agg.run > agg.best {
				agg.best = agg.run
			}
		case childScore == parentScore:
			agg.draws++
			agg.run = 0
		default:
			agg.losses++
			agg.run = 0
		}
		agg.current = agg.run
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	items := make([]RankingEntry, 0, len(order))
	for _, id := range order {
		info := children[id]
		agg := stats[id]
		items = append(items, RankingEntry{
			ChildID: id, ChildName: info.name,
			Battles: agg.battles, Wins: agg.wins, Draws: agg.draws, Losses: agg.losses,
			Points:        agg.wins*3 + agg.draws,
			CurrentStreak: agg.current, BestStreak: agg.best,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Points != items[j].Points {
			return items[i].Points > items[j].Points
		}
		if items[i].Wins != items[j].Wins {
			return items[i].Wins > items[j].Wins
		}
		return items[i].ChildName < items[j].ChildName
	})
	for i := range items {
		item := &items[i]
		medals := make([]string, 0, 5)
		if item.Wins > 0 {
			medals = append(medals, "first_win")
		}
		if item.BestStreak >= 3 {
			medals = append(medals, "streak_3")
		}
		if item.Battles >= 10 {
			medals = append(medals, "veteran")
		}
		if item.Battles >= 3 && item.Wins == item.Battles {
			medals = append(medals, "all_win")
		}
		if i == 0 && item.Battles > 0 {
			medals = append(medals, "champion")
		}
		item.Medals = medals
	}
	return items, nil
}

// History 返回孩子最近的对战摘要。
func (s *Service) History(ctx context.Context, accountID, childID string, limit int) ([]Summary, error) {
	if _, err := s.ownsChild(ctx, accountID, childID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		select id, mode, status, jsonb_array_length(question_ids),
		       child_score, parent_score, created_at, finished_at
		from family_battle where child_id = $1
		order by created_at desc limit $2`, childID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Summary, 0, limit)
	for rows.Next() {
		var item Summary
		if err := rows.Scan(&item.ID, &item.Mode, &item.Status, &item.QuestionCount,
			&item.ChildScore, &item.ParentScore, &item.CreatedAt, &item.FinishedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// Create 创建一个对局；room 模式返回房间码并等待加入。
func (s *Service) Create(ctx context.Context, accountID, childID string, count int, mode string) (State, error) {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "same_device"
	}
	if mode != "same_device" && mode != "room" {
		return State{}, apiError{422, "VALIDATION_ERROR", "对战模式不合法"}
	}
	familyID, err := s.ownsChild(ctx, accountID, childID)
	if err != nil {
		return State{}, err
	}
	stage := ""
	_ = s.pool.QueryRow(ctx, `select stage_code from child_profile where id = $1`, childID).Scan(&stage)

	rows, err := s.pool.Query(ctx, `
		select id from question
		where status = 'online' and type_code in ('choice', 'image') and options is not null
		order by (stage_code = $1) desc, random()
		limit $2`, stage, count)
	if err != nil {
		return State{}, err
	}
	defer rows.Close()
	ids := make([]string, 0, count)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return State{}, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return State{}, err
	}
	if len(ids) < count {
		return State{}, apiError{409, "QUESTION_POOL_EMPTY", "题库可用题目不足，无法发起对战"}
	}

	encoded, err := json.Marshal(ids)
	if err != nil {
		return State{}, err
	}
	status := "child_turn"
	joinCode := ""
	if mode == "room" {
		status = "waiting"
		joinCode = randomCode()
	}
	var battleID string
	err = s.pool.QueryRow(ctx, `
		insert into family_battle (
			family_id, child_id, question_ids, created_by, mode, status, join_code,
			join_code_expires_at)
		values (
			$1, $2, $3::jsonb, nullif($4, '')::uuid, $5, $6, nullif($7, ''),
			case when $5 = 'room' then now() + ($8::int * interval '1 second') else null end)
		returning id`,
		familyID, childID, string(encoded), accountID, mode, status, joinCode,
		int(roomCodeTTL.Seconds())).Scan(&battleID)
	if err != nil {
		return State{}, err
	}
	return s.Get(ctx, childID, battleID)
}

// Join 通过房间码加入房间；重复加入幂等。
func (s *Service) Join(ctx context.Context, accountID, childID, code string) (State, error) {
	familyID, err := s.ownsChild(ctx, accountID, childID)
	if err != nil {
		return State{}, err
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 6 {
		return State{}, apiError{422, "VALIDATION_ERROR", "房间码为 6 位"}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return State{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var battleID, status string
	var expiresAt *time.Time
	err = tx.QueryRow(ctx, `
		select id, status, join_code_expires_at from family_battle
		where join_code = $1 and child_id = $2 and family_id = $3 for update`,
		code, childID, familyID).Scan(&battleID, &status, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return State{}, apiError{404, "BATTLE_NOT_FOUND", "房间不存在或已失效"}
	}
	if err != nil {
		return State{}, err
	}
	if status == "waiting" {
		if expiresAt != nil && !expiresAt.After(time.Now()) {
			return State{}, apiError{409, "ROOM_EXPIRED", "房间码已过期"}
		}
		if _, err := tx.Exec(ctx,
			`update family_battle set status = 'child_turn' where id = $1`, battleID); err != nil {
			return State{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return State{}, err
	}
	s.events.publish(ctx, battleID)
	return s.Get(ctx, childID, battleID)
}

// Get 返回对局状态；未结束时不下发答案与解析。
func (s *Service) Get(ctx context.Context, childID, battleID string) (State, error) {
	battle, err := s.load(ctx, childID, battleID)
	if err != nil {
		return State{}, err
	}
	questions, err := s.questions(ctx, battle.QuestionIDs)
	if err != nil {
		return State{}, err
	}
	state := State{
		ID:            battle.ID,
		Status:        battle.Status,
		QuestionCount: len(battle.QuestionIDs),
		Questions:     questions,
		ChildScore:    battle.ChildScore,
		ParentScore:   battle.ParentScore,
		Mode:          battle.Mode,
	}
	if battle.Mode == "room" && battle.Status != "finished" {
		state.JoinCode = battle.JoinCode
		state.JoinCodeExpiresAt = battle.JoinCodeExpiresAt
	}
	if battle.Status == "finished" {
		state.Result = &ResultView{
			Winner:      winner(battle.ChildScore, battle.ParentScore),
			ChildScore:  battle.ChildScore,
			ParentScore: battle.ParentScore,
		}
		answers, err := s.answers(ctx, battle.ID)
		if err != nil {
			return State{}, err
		}
		state.Answers = answers
	}
	return state, nil
}

// Submit 提交当前一方答案并推进状态。
func (s *Service) Submit(ctx context.Context, childID, battleID, side string, submissions []Submission) (State, error) {
	if side != "child" && side != "parent" {
		return State{}, apiError{422, "VALIDATION_ERROR", "对战方不合法"}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return State{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	var rawIDs string
	var childScore, parentScore int
	err = tx.QueryRow(ctx, `
		select status, question_ids::text, child_score, parent_score
		from family_battle where id = $1 and child_id = $2 for update`,
		battleID, childID).Scan(&status, &rawIDs, &childScore, &parentScore)
	if errors.Is(err, pgx.ErrNoRows) {
		return State{}, apiError{404, "BATTLE_NOT_FOUND", "对局不存在"}
	}
	if err != nil {
		return State{}, err
	}

	if status == "waiting" {
		return State{}, apiError{409, "BATTLE_NOT_JOINED", "对战房间尚未加入"}
	}
	expected := "child"
	if status == "parent_turn" {
		expected = "parent"
	}
	if status == "finished" || side != expected {
		return State{}, apiError{409, "BATTLE_TURN_INVALID", "当前不是该方答题轮次"}
	}

	var questionIDs []string
	if err := json.Unmarshal([]byte(rawIDs), &questionIDs); err != nil {
		return State{}, err
	}
	items, err := s.questions(ctx, questionIDs)
	if err != nil {
		return State{}, err
	}
	correctAnswers := map[string]json.RawMessage{}
	for _, q := range items {
		rec, err := s.question(ctx, q.ID)
		if err != nil {
			return State{}, err
		}
		correctAnswers[q.ID] = rec.Answer
	}

	if len(submissions) != len(questionIDs) {
		return State{}, apiError{422, "VALIDATION_ERROR", "需要提交全部题目答案"}
	}
	seen := map[string]bool{}
	correct := 0
	for position, item := range submissions {
		if item.QuestionID == "" || seen[item.QuestionID] {
			return State{}, apiError{422, "VALIDATION_ERROR", "答案重复或缺少题目"}
		}
		expectedAnswer, ok := correctAnswers[item.QuestionID]
		if !ok {
			return State{}, apiError{422, "VALIDATION_ERROR", "答案包含不属于本局的题目"}
		}
		seen[item.QuestionID] = true
		ok = equalSet(expectedAnswer, item.Answer)
		if ok {
			correct++
		}
		if _, err := tx.Exec(ctx, `
			insert into family_battle_answer (battle_id, side, question_id, answer, correct, position)
			values ($1, $2, $3, $4::jsonb, $5, $6)`,
			battleID, side, item.QuestionID, string(item.Answer), ok, position); err != nil {
			return State{}, err
		}
	}

	nextStatus := "parent_turn"
	if side == "parent" {
		nextStatus = "finished"
	}
	if side == "child" {
		childScore = correct
	} else {
		parentScore = correct
	}
	if _, err := tx.Exec(ctx, `
		update family_battle
		set status = $2, child_score = $3, parent_score = $4,
		    finished_at = case when $2 = 'finished' then now() else finished_at end
		where id = $1`, battleID, nextStatus, childScore, parentScore); err != nil {
		return State{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return State{}, err
	}
	s.events.publish(ctx, battleID)
	return s.Get(ctx, childID, battleID)
}

func (s *Service) ownsChild(ctx context.Context, accountID, childID string) (string, error) {
	if accountID == "" {
		return "", apiError{401, "AUTH_REQUIRED", "家长未登录"}
	}
	var familyID string
	err := s.pool.QueryRow(ctx, `
		select cp.family_id::text from child_profile cp
		join family_member fm on fm.family_id = cp.family_id
		where cp.id = $1 and fm.account_id = $2
		  and cp.family_id = coalesce(
		      (select active_family_id from parent_account where id = $2), fm.family_id)`,
		childID, accountID).Scan(&familyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apiError{404, "CHILD_NOT_FOUND", "孩子档案不存在"}
	}
	if err != nil {
		return "", err
	}
	return familyID, nil
}

func (s *Service) load(ctx context.Context, childID, battleID string) (battleRecord, error) {
	var rec battleRecord
	var rawIDs string
	err := s.pool.QueryRow(ctx, `
		select id, family_id, status, mode, coalesce(join_code, ''),
		       join_code_expires_at, question_ids::text, child_score, parent_score
		from family_battle where id = $1 and child_id = $2`,
		battleID, childID).Scan(&rec.ID, &rec.FamilyID, &rec.Status, &rec.Mode,
		&rec.JoinCode, &rec.JoinCodeExpiresAt, &rawIDs, &rec.ChildScore, &rec.ParentScore)
	if errors.Is(err, pgx.ErrNoRows) {
		return battleRecord{}, apiError{404, "BATTLE_NOT_FOUND", "对局不存在"}
	}
	if err != nil {
		return battleRecord{}, err
	}
	if err := json.Unmarshal([]byte(rawIDs), &rec.QuestionIDs); err != nil {
		return battleRecord{}, err
	}
	return rec, nil
}

func (s *Service) questions(ctx context.Context, ids []string) ([]QuestionView, error) {
	items := make([]QuestionView, 0, len(ids))
	for _, id := range ids {
		rec, err := s.question(ctx, id)
		if err != nil {
			return nil, err
		}
		items = append(items, QuestionView{
			ID: rec.ID, Stem: rec.Stem, Type: rec.Type, Options: rec.Options,
		})
	}
	return items, nil
}

func (s *Service) question(ctx context.Context, id string) (questionRecord, error) {
	var rec questionRecord
	err := s.pool.QueryRow(ctx, `
		select id, stem, type_code, coalesce(options::text, '[]'),
		       answer::text, coalesce(explain, '')
		from question where id = $1`, id).
		Scan(&rec.ID, &rec.Stem, &rec.Type, &rec.Options, &rec.Answer, &rec.Explain)
	if errors.Is(err, pgx.ErrNoRows) {
		return questionRecord{}, apiError{404, "QUESTION_NOT_FOUND", "题目不存在"}
	}
	return rec, err
}

func (s *Service) answers(ctx context.Context, battleID string) ([]AnswerView, error) {
	rows, err := s.pool.Query(ctx, `
		select a.side, a.question_id, a.answer::text, a.correct,
		       q.answer::text, coalesce(q.explain, '')
		from family_battle_answer a
		join question q on q.id = a.question_id
		where a.battle_id = $1
		order by a.position, a.side`, battleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AnswerView, 0)
	for rows.Next() {
		var item AnswerView
		var answer, correctAnswer string
		if err := rows.Scan(&item.Side, &item.QuestionID, &answer, &item.Correct,
			&correctAnswer, &item.Explain); err != nil {
			return nil, err
		}
		item.Answer = json.RawMessage(answer)
		item.CorrectAnswer = json.RawMessage(correctAnswer)
		items = append(items, item)
	}
	return items, rows.Err()
}

func winner(child, parent int) string {
	switch {
	case child > parent:
		return "child"
	case parent > child:
		return "parent"
	default:
		return "draw"
	}
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
