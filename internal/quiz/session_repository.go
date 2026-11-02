package quiz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type sessionRecord struct {
	ID            string
	ChildID       string
	DeviceID      string
	State         string
	RuleVersion   int
	PassThreshold float64
	Score         float64
	CorrectCount  int
	QuestionCount int
	RuleSnapshot  []byte
	StartedAt     time.Time
	ExpiresAt     time.Time
	FinishedAt    *time.Time
}

type itemRecord struct {
	ID              string
	QuestionID      string
	QuestionVersion int
	Position        int
	Token           string
	Snapshot        []byte
	IssuedAt        time.Time
	AnsweredAt      *time.Time
	IsCorrect       *bool
	DurationMS      *int
}

type questionDetail struct {
	ID            string
	Version       int
	Subject       string
	Type          string
	Stem          string
	StemMedia     json.RawMessage
	Options       json.RawMessage
	Answer        json.RawMessage
	Alias         json.RawMessage
	Explain       string
	DurationLimit *int
}

func (r *repository) ruleForChild(ctx context.Context, childID string) (ruleSnapshot, error) {
	var version, unlockMinutes, funCap int
	var threshold float64
	var weightsJSON string
	var funEnabled bool
	err := r.pool.QueryRow(ctx, `
		select version, unlock_minutes, pass_threshold, subject_weight::text,
		       fun_ratio_cap, fun_enabled
		from watch_rule where child_id = $1`, childID).Scan(
		&version, &unlockMinutes, &threshold, &weightsJSON, &funCap, &funEnabled)
	if err != nil {
		return ruleSnapshot{}, err
	}
	weights := map[string]float64{}
	if err := json.Unmarshal([]byte(weightsJSON), &weights); err != nil {
		return ruleSnapshot{}, fmt.Errorf("decode subject weights: %w", err)
	}
	return ruleSnapshot{Version: version, Rule: Rule{
		UnlockMinutes: unlockMinutes,
		PassThreshold: threshold,
		SubjectWeight: weights,
		FunRatioCap:   funCap,
		FunEnabled:    funEnabled,
	}}, nil
}

func (r *repository) activeSessionForDevice(ctx context.Context, deviceID string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		select id from quiz_session
		where device_id = $1 and state = 'active' and expires_at > now()
		order by started_at desc limit 1`, deviceID).Scan(&id)
	return id, err
}

func (r *repository) createSession(ctx context.Context, childID, deviceID string, rule ruleSnapshot,
	threshold float64, expiresAt time.Time, items []itemRecord) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ruleJSON, _ := json.Marshal(rule)
	var sessionID string
	if err := tx.QueryRow(ctx, `
		insert into quiz_session (
			child_id, device_id, rule_version, state, question_count, pass_threshold,
			rule_snapshot, expires_at)
		values ($1, $2, $3, 'active', $4, $5, $6::jsonb, $7)
		returning id`,
		childID, deviceID, rule.Version, len(items), threshold, string(ruleJSON), expiresAt).
		Scan(&sessionID); err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}

	for _, item := range items {
		snapshot := item.Snapshot
		var snap itemSnapshot
		_ = json.Unmarshal(snapshot, &snap)
		if _, err := tx.Exec(ctx, `
			insert into quiz_session_item (
				session_id, question_id, question_version, answer_snapshot,
				question_snapshot, position, answer_token_jti)
			values ($1, $2, $3, $4::jsonb, $5::jsonb, $6, $7)`,
			sessionID, item.QuestionID, item.QuestionVersion,
			jsonText(snap.Answer), string(snapshot), item.Position, item.Token); err != nil {
			return "", fmt.Errorf("insert session item: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return sessionID, nil
}

func jsonText(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

func (r *repository) questionDetails(ctx context.Context, ids []string) (map[string]questionDetail, error) {
	rows, err := r.pool.Query(ctx, `
		select id, version, subject_code, type_code, stem,
		       coalesce(stem_media::text, ''), coalesce(options::text, ''),
		       answer::text, coalesce(answer_alias::text, ''),
		       coalesce(explain, ''), duration_limit
		from question
		where id = any($1) and status = 'online'`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	details := make(map[string]questionDetail, len(ids))
	for rows.Next() {
		var d questionDetail
		var media, options, answer, alias string
		if err := rows.Scan(&d.ID, &d.Version, &d.Subject, &d.Type, &d.Stem,
			&media, &options, &answer, &alias, &d.Explain, &d.DurationLimit); err != nil {
			return nil, err
		}
		d.StemMedia = rawOrEmpty(media)
		d.Options = rawOrEmpty(options)
		d.Answer = json.RawMessage(answer)
		d.Alias = rawOrEmpty(alias)
		details[d.ID] = d
	}
	return details, rows.Err()
}

func rawOrEmpty(value string) json.RawMessage {
	if value == "" {
		return nil
	}
	return json.RawMessage(value)
}

func (r *repository) sessionByID(ctx context.Context, sessionID string) (sessionRecord, error) {
	var rec sessionRecord
	var ruleJSON string
	err := r.pool.QueryRow(ctx, `
		select id, child_id, device_id, state, rule_version, pass_threshold, score,
		       correct_count, question_count, rule_snapshot::text,
		       started_at, expires_at, finished_at
		from quiz_session where id = $1`, sessionID).Scan(
		&rec.ID, &rec.ChildID, &rec.DeviceID, &rec.State, &rec.RuleVersion,
		&rec.PassThreshold, &rec.Score, &rec.CorrectCount, &rec.QuestionCount,
		&ruleJSON, &rec.StartedAt, &rec.ExpiresAt, &rec.FinishedAt)
	if err != nil {
		return sessionRecord{}, err
	}
	rec.RuleSnapshot = []byte(ruleJSON)
	return rec, nil
}

func (r *repository) itemsBySession(ctx context.Context, sessionID string) ([]itemRecord, error) {
	rows, err := r.pool.Query(ctx, `
		select id, question_id, question_version, position, answer_token_jti,
		       question_snapshot::text, issued_at, answered_at, is_correct, duration_ms
		from quiz_session_item
		where session_id = $1
		order by position`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]itemRecord, 0)
	for rows.Next() {
		var item itemRecord
		var snapshot string
		if err := rows.Scan(&item.ID, &item.QuestionID, &item.QuestionVersion, &item.Position,
			&item.Token, &snapshot, &item.IssuedAt, &item.AnsweredAt, &item.IsCorrect, &item.DurationMS); err != nil {
			return nil, err
		}
		item.Snapshot = []byte(snapshot)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *repository) itemByToken(ctx context.Context, sessionID, token string) (itemRecord, error) {
	var item itemRecord
	var snapshot string
	err := r.pool.QueryRow(ctx, `
		select id, question_id, question_version, position, answer_token_jti,
		       question_snapshot::text, issued_at, answered_at, is_correct, duration_ms
		from quiz_session_item
		where session_id = $1 and answer_token_jti = $2`, sessionID, token).Scan(
		&item.ID, &item.QuestionID, &item.QuestionVersion, &item.Position,
		&item.Token, &snapshot, &item.IssuedAt, &item.AnsweredAt, &item.IsCorrect, &item.DurationMS)
	if err != nil {
		return itemRecord{}, err
	}
	item.Snapshot = []byte(snapshot)
	return item, nil
}

func (r *repository) markAnswered(ctx context.Context, itemID string, given json.RawMessage, correct bool, durationMS int) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		update quiz_session_item
		set answered_at = now(), answer_given = $2::jsonb, is_correct = $3, duration_ms = $4
		where id = $1 and answered_at is null`,
		itemID, string(given), correct, durationMS)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *repository) addScore(ctx context.Context, sessionID string, delta float64, correctDelta int) (float64, int, error) {
	var score float64
	var correct int
	err := r.pool.QueryRow(ctx, `
		update quiz_session
		set score = score + $2, correct_count = correct_count + $3
		where id = $1 and state = 'active'
		returning score, correct_count`, sessionID, delta, correctDelta).Scan(&score, &correct)
	return score, correct, err
}

func (r *repository) insertAnswerLog(ctx context.Context, childID, questionID, sessionID, deviceID string,
	given json.RawMessage, correct bool, durationMS int) error {
	_, err := r.pool.Exec(ctx, `
		insert into answer_log (child_id, question_id, session_id, device_id, answer_given, is_correct, duration_ms)
		values ($1, $2, $3, $4, $5::jsonb, $6, $7)`,
		childID, questionID, sessionID, deviceID, string(given), correct, durationMS)
	return err
}

func (r *repository) setSessionState(ctx context.Context, sessionID, state string) error {
	_, err := r.pool.Exec(ctx, `
		update quiz_session
		set state = $2, finished_at = now()
		where id = $1 and state = 'active'`, sessionID, state)
	return err
}

func (r *repository) answeredCount(ctx context.Context, sessionID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		select count(*) from quiz_session_item
		where session_id = $1 and answered_at is not null`, sessionID).Scan(&count)
	return count, err
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
