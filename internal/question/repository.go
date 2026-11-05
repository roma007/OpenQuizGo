package question

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const questionColumns = `
	id, subject_code, category_code, stage_code, coalesce(grade_tag, ''), difficulty, type_code, stem,
	coalesce(stem_media::text, ''), coalesce(options::text, ''), answer::text, coalesce(answer_alias::text, ''),
	coalesce(explain, ''), duration_limit, coalesce(topic_id::text, ''), status, version, schema_ver,
	created_at, updated_at`

type repository struct {
	pool *pgxpool.Pool
}

func newRepository(pool *pgxpool.Pool) *repository {
	return &repository{pool: pool}
}

func scanQuestion(row pgx.Row) (Question, error) {
	var q Question
	var stemMedia, options, answer, alias string
	err := row.Scan(
		&q.ID, &q.Subject, &q.Category, &q.Stage, &q.GradeTag, &q.Difficulty, &q.Type, &q.Stem,
		&stemMedia, &options, &answer, &alias, &q.Explain, &q.DurationLimit, &q.TopicID,
		&q.Status, &q.Version, &q.SchemaVer, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return Question{}, err
	}
	q.StemMedia = rawOrNil(stemMedia)
	q.Options = rawOrNil(options)
	q.Answer = rawOrNil(answer)
	q.AnswerAlias = rawOrNil(alias)
	return q, nil
}

func rawOrNil(value string) json.RawMessage {
	if value == "" {
		return nil
	}
	return json.RawMessage(value)
}

func jsonParam(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

func (r *repository) create(ctx context.Context, in Input, adminID string) (Question, error) {
	const query = `
		insert into question (
			subject_code, category_code, stage_code, grade_tag, difficulty, type_code, stem,
			stem_media, options, answer, answer_alias, explain, duration_limit, topic_id,
			status, version, schema_ver, created_by)
		values ($1,$2,$3,nullif($4,''),$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb,$11::jsonb,
		        nullif($12,''),$13,nullif($14,'')::uuid,'draft',1,$15,$16)
		returning ` + questionColumns

	return scanQuestion(r.pool.QueryRow(ctx, query,
		in.Subject, in.Category, in.Stage, in.GradeTag, in.Difficulty, in.Type, in.Stem,
		jsonParam(in.StemMedia), jsonParam(in.Options), jsonParam(in.Answer), jsonParam(in.AnswerAlias),
		in.Explain, in.DurationLimit, topicParam(in.TopicID), in.SchemaVer, adminID))
}

func topicParam(id *string) any {
	if id == nil || *id == "" {
		return nil
	}
	return *id
}

func (r *repository) update(ctx context.Context, id string, in Input, adminID string) (Question, error) {
	const query = `
		update question set
			subject_code = $2, category_code = $3, stage_code = $4, grade_tag = nullif($5,''),
			difficulty = $6, type_code = $7, stem = $8,
			stem_media = $9::jsonb, options = $10::jsonb, answer = $11::jsonb, answer_alias = $12::jsonb,
			explain = nullif($13,''), duration_limit = $14, topic_id = nullif($15,'')::uuid,
			schema_ver = $16, version = version + 1, updated_at = now()
		where id = $1 and status in ('draft', 'offline')
		returning ` + questionColumns

	return scanQuestion(r.pool.QueryRow(ctx, query, id,
		in.Subject, in.Category, in.Stage, in.GradeTag, in.Difficulty, in.Type, in.Stem,
		jsonParam(in.StemMedia), jsonParam(in.Options), jsonParam(in.Answer), jsonParam(in.AnswerAlias),
		in.Explain, in.DurationLimit, topicParam(in.TopicID), in.SchemaVer))
}

func (r *repository) get(ctx context.Context, id string) (Question, error) {
	return scanQuestion(r.pool.QueryRow(ctx, `select `+questionColumns+` from question where id = $1`, id))
}

type calibrationRow struct {
	ID         string
	Stem       string
	Subject    string
	Stage      string
	Difficulty int
	Answered   int
	Correct    int
}

// calibrationRows 返回达到样本门槛的在线题统计。
func (r *repository) calibrationRows(ctx context.Context, minAnswers, limit int) ([]calibrationRow, error) {
	rows, err := r.pool.Query(ctx, `
		select q.id, q.stem, q.subject_code, q.stage_code, q.difficulty,
		       count(a.id)::int, count(*) filter (where a.is_correct)::int
		from question q
		join answer_log a on a.question_id = q.id
		where q.status = 'online'
		group by q.id, q.stem, q.subject_code, q.stage_code, q.difficulty
		having count(a.id) >= $1
		order by count(a.id) desc, q.id
		limit $2`, minAnswers, limit)
	if err != nil {
		return nil, fmt.Errorf("calibration query: %w", err)
	}
	defer rows.Close()

	result := make([]calibrationRow, 0)
	for rows.Next() {
		var row calibrationRow
		if err := rows.Scan(&row.ID, &row.Stem, &row.Subject, &row.Stage,
			&row.Difficulty, &row.Answered, &row.Correct); err != nil {
			return nil, fmt.Errorf("scan calibration: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// applyCalibration 批量更新难度并写审计。
func (r *repository) applyCalibration(ctx context.Context, items []CalibrationApplyItem,
	adminID, ip string) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	applied := 0
	for _, item := range items {
		var before int
		err := tx.QueryRow(ctx, `
			select difficulty from question
			where id = $1 and status = 'online' for update`, item.QuestionID).Scan(&before)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("lock question: %w", err)
		}
		if before == item.Difficulty {
			continue
		}
		if _, err := tx.Exec(ctx, `
			update question set difficulty = $2, version = version + 1, updated_at = now()
			where id = $1`, item.QuestionID, item.Difficulty); err != nil {
			return 0, fmt.Errorf("update difficulty: %w", err)
		}
		after, _ := json.Marshal(map[string]int{"from": before, "to": item.Difficulty})
		if _, err := tx.Exec(ctx, `
			insert into audit_log (actor_type, actor_id, action, target_type, target_id, after_data, ip)
			values ('admin', $1, 'question.calibrate', 'question', $2, $3::jsonb, nullif($4, '')::inet)`,
			adminID, item.QuestionID, string(after), ip); err != nil {
			return 0, fmt.Errorf("audit calibration: %w", err)
		}
		applied++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return applied, nil
}

// calibrationHistory 从审计日志读取难度校准历史；after_data 缺少 from/to 的记录跳过。
func (r *repository) calibrationHistory(ctx context.Context, questionID string, limit int) ([]CalibrationHistoryItem, error) {
	query := `
		select id, target_id, (after_data->>'from')::int, (after_data->>'to')::int,
		       actor_id, created_at
		from audit_log
		where action = 'question.calibrate' and target_type = 'question'
		  and jsonb_typeof(after_data->'from') = 'number'
		  and jsonb_typeof(after_data->'to') = 'number'`
	args := make([]any, 0, 2)
	if questionID != "" {
		args = append(args, questionID)
		query += fmt.Sprintf(" and target_id = $%d", len(args))
	}
	args = append(args, limit)
	query += fmt.Sprintf(" order by id desc limit $%d", len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("calibration history: %w", err)
	}
	defer rows.Close()

	items := make([]CalibrationHistoryItem, 0)
	for rows.Next() {
		var item CalibrationHistoryItem
		if err := rows.Scan(&item.ID, &item.QuestionID, &item.From, &item.To,
			&item.ActorID, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan calibration history: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// Coverage 聚合题库分布。
func (r *repository) Coverage(ctx context.Context) ([]CoverageCell, int, error) {
	rows, err := r.pool.Query(ctx, `
		select subject_code, stage_code, type_code, status, difficulty, count(*)
		from question
		group by subject_code, stage_code, type_code, status, difficulty
		order by subject_code, stage_code, type_code, status, difficulty`)
	if err != nil {
		return nil, 0, fmt.Errorf("coverage query: %w", err)
	}
	defer rows.Close()

	cells := make([]CoverageCell, 0)
	total := 0
	for rows.Next() {
		var cell CoverageCell
		if err := rows.Scan(&cell.Subject, &cell.Stage, &cell.Type, &cell.Status,
			&cell.Difficulty, &cell.Count); err != nil {
			return nil, 0, fmt.Errorf("scan coverage: %w", err)
		}
		total += cell.Count
		cells = append(cells, cell)
	}
	return cells, total, rows.Err()
}

func (r *repository) list(ctx context.Context, f Filter) (ListResult, error) {
	where := []string{"true"}
	args := make([]any, 0, 8)

	add := func(expr string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(expr, len(args)))
	}
	if f.Subject != "" {
		add("subject_code = $%d", f.Subject)
	}
	if f.Category != "" {
		add("category_code = $%d", f.Category)
	}
	if f.Stage != "" {
		add("stage_code = $%d", f.Stage)
	}
	if f.Difficulty > 0 {
		add("difficulty = $%d", f.Difficulty)
	}
	if f.Type != "" {
		add("type_code = $%d", f.Type)
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.TopicID != "" {
		add("topic_id = $%d::uuid", f.TopicID)
	}
	if f.Keyword != "" {
		add("stem ilike '%%' || $%d || '%%'", f.Keyword)
	}
	clause := strings.Join(where, " and ")

	var total int
	if err := r.pool.QueryRow(ctx, `select count(*) from question where `+clause, args...).Scan(&total); err != nil {
		return ListResult{}, fmt.Errorf("count questions: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	query := `select ` + questionColumns + ` from question where ` + clause +
		fmt.Sprintf(" order by updated_at desc, id desc limit $%d offset $%d", len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("query questions: %w", err)
	}
	defer rows.Close()

	items := make([]Question, 0)
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, q)
	}
	return ListResult{Items: items, Total: total}, rows.Err()
}

func (r *repository) transition(ctx context.Context, id, from, to, adminID string) (Question, error) {
	const query = `
		update question set
			status = $2,
			updated_at = now(),
			reviewed_by = case when $2 in ('trial', 'online') then $3::uuid else reviewed_by end
		where id = $1 and status = $4
		returning ` + questionColumns
	return scanQuestion(r.pool.QueryRow(ctx, query, id, to, adminID, from))
}

func (r *repository) deleteDraft(ctx context.Context, id string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `delete from question where id = $1 and status = 'draft'`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *repository) insertMany(ctx context.Context, inputs []Input, adminID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `
		insert into question (
			subject_code, category_code, stage_code, grade_tag, difficulty, type_code, stem,
			stem_media, options, answer, answer_alias, explain, duration_limit, topic_id,
			status, version, schema_ver, created_by)
		values ($1,$2,$3,nullif($4,''),$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb,$11::jsonb,
		        nullif($12,''),$13,nullif($14,'')::uuid,'draft',1,$15,$16)`

	batch := &pgx.Batch{}
	for _, in := range inputs {
		batch.Queue(query,
			in.Subject, in.Category, in.Stage, in.GradeTag, in.Difficulty, in.Type, in.Stem,
			jsonParam(in.StemMedia), jsonParam(in.Options), jsonParam(in.Answer), jsonParam(in.AnswerAlias),
			in.Explain, in.DurationLimit, topicParam(in.TopicID), in.SchemaVer, adminID)
	}
	results := tx.SendBatch(ctx, batch)
	for range inputs {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("insert question: %w", err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close batch: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *repository) audit(ctx context.Context, actorID, action, targetID string, after any, ip string) error {
	var payload any
	if after != nil {
		encoded, err := json.Marshal(after)
		if err != nil {
			return err
		}
		payload = string(encoded)
	}
	_, err := r.pool.Exec(ctx, `
		insert into audit_log (actor_type, actor_id, action, target_type, target_id, after_data, ip)
		values ('admin', $1, $2, 'question', $3, $4::jsonb, nullif($5, '')::inet)`,
		actorID, action, targetID, payload, ip)
	return err
}

func (r *repository) listAudit(ctx context.Context, filter AuditFilter) ([]AuditEntry, error) {
	query := `
		select id, actor_type, actor_id, action, target_type, target_id,
		       coalesce(after_data::text, ''), created_at
		from audit_log where 1 = 1`
	args := make([]any, 0, 5)
	add := func(field, value string) {
		if value == "" {
			return
		}
		args = append(args, value)
		query += fmt.Sprintf(" and %s = $%d", field, len(args))
	}
	add("target_id", filter.TargetID)
	add("target_type", filter.TargetType)
	add("action", filter.Action)
	add("actor_id", filter.ActorID)
	if filter.From != nil {
		args = append(args, *filter.From)
		query += fmt.Sprintf(" and created_at >= $%d", len(args))
	}
	if filter.To != nil {
		args = append(args, filter.To.AddDate(0, 0, 1))
		query += fmt.Sprintf(" and created_at < $%d", len(args))
	}
	args = append(args, filter.Limit)
	query += fmt.Sprintf(" order by id desc limit $%d", len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]AuditEntry, 0)
	for rows.Next() {
		var e AuditEntry
		var after string
		if err := rows.Scan(&e.ID, &e.ActorType, &e.ActorID, &e.Action, &e.TargetType,
			&e.TargetID, &after, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.After = rawOrNil(after)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

type tagSets struct {
	subjects   map[string]bool
	categories map[string]bool
	stages     map[string]bool
	types      map[string]bool
}

func (r *repository) tags(ctx context.Context) (tagSets, error) {
	rows, err := r.pool.Query(ctx, `select tag_type, code from tag_dict where enabled = true`)
	if err != nil {
		return tagSets{}, err
	}
	defer rows.Close()

	sets := tagSets{
		subjects:   map[string]bool{},
		categories: map[string]bool{},
		stages:     map[string]bool{},
		types:      map[string]bool{},
	}
	for rows.Next() {
		var kind, code string
		if err := rows.Scan(&kind, &code); err != nil {
			return tagSets{}, err
		}
		switch kind {
		case "subject":
			sets.subjects[code] = true
		case "category":
			sets.categories[code] = true
		case "stage":
			sets.stages[code] = true
		case "type":
			sets.types[code] = true
		}
	}
	return sets, rows.Err()
}

func (r *repository) topicExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`select exists (select 1 from topic_pack where id = $1)`, id).Scan(&exists)
	return exists, err
}

func (r *repository) topicIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := r.pool.Query(ctx, `select id::text from topic_pack`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

var errNoRows = pgx.ErrNoRows

func isNoRows(err error) bool {
	return errors.Is(err, errNoRows)
}
