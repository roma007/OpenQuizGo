package report

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	pool *pgxpool.Pool
}

func newRepository(pool *pgxpool.Pool) *repository {
	return &repository{pool: pool}
}

type answerRow struct {
	Date     string
	Subject  string
	Answered int
	Correct  int
}

func (r *repository) aggregateAnswers(ctx context.Context, childID string, start, end time.Time, timezone string) ([]answerRow, error) {
	rows, err := r.pool.Query(ctx, `
		select to_char((a.answered_at at time zone $4)::date, 'YYYY-MM-DD') as day,
		       q.subject_code,
		       count(*)::int,
		       sum(case when a.is_correct then 1 else 0 end)::int
		from answer_log a
		join question q on q.id = a.question_id
		where a.child_id = $1 and a.answered_at >= $2 and a.answered_at < $3
		group by 1, 2
		order by 1`, childID, start, end, timezone)
	if err != nil {
		return nil, fmt.Errorf("aggregate answers: %w", err)
	}
	defer rows.Close()

	items := make([]answerRow, 0)
	for rows.Next() {
		var row answerRow
		if err := rows.Scan(&row.Date, &row.Subject, &row.Answered, &row.Correct); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (r *repository) watchMinutes(ctx context.Context, childID string, start, end time.Time) (int, error) {
	var seconds int
	err := r.pool.QueryRow(ctx, `
		select coalesce(sum(watched_seconds), 0)::int
		from watch_log
		where child_id = $1 and start_at >= $2 and start_at < $3`,
		childID, start, end).Scan(&seconds)
	if err != nil {
		return 0, err
	}
	return (seconds + 30) / 60, nil
}

func (r *repository) upsert(ctx context.Context, childID, period string, start time.Time,
	subjectStats, funStats map[string]any, watchMinutes int) (string, time.Time, error) {
	subjects, _ := json.Marshal(subjectStats)
	fun, _ := json.Marshal(funStats)

	var id string
	var generatedAt time.Time
	err := r.pool.QueryRow(ctx, `
		insert into growth_report (child_id, period, period_start, subject_stats, fun_stats, watch_minutes, generated_at)
		values ($1, $2, $3, $4::jsonb, $5::jsonb, $6, now())
		on conflict (child_id, period, period_start) do update set
			subject_stats = excluded.subject_stats,
			fun_stats = excluded.fun_stats,
			watch_minutes = excluded.watch_minutes,
			generated_at = now()
		returning id, generated_at`,
		childID, period, start, string(subjects), string(fun), watchMinutes).Scan(&id, &generatedAt)
	return id, generatedAt, err
}

func (r *repository) childTimezone(ctx context.Context, childID string) (string, error) {
	var timezone string
	err := r.pool.QueryRow(ctx, `
		select coalesce(wr.timezone, 'Asia/Shanghai')
		from child_profile c
		left join watch_rule wr on wr.child_id = c.id
		where c.id = $1`, childID).Scan(&timezone)
	return timezone, err
}

func (r *repository) list(ctx context.Context, accountID, childID, period string, limit int) ([]Report, error) {
	rows, err := r.pool.Query(ctx, `
		select gr.id, gr.child_id, gr.period, gr.period_start,
		       gr.subject_stats::text, gr.fun_stats::text, gr.watch_minutes, gr.generated_at
		from growth_report gr
		where gr.child_id = $1 and gr.period = $2
		  and exists (
		      select 1 from child_profile c
		      join family_member fm on fm.family_id = c.family_id
		      where c.id = gr.child_id and fm.account_id = $3
		  )
		order by gr.period_start desc
		limit $4`, childID, period, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := make([]Report, 0)
	for rows.Next() {
		var rep Report
		var subjects, fun string
		if err := rows.Scan(&rep.ID, &rep.ChildID, &rep.Period, &rep.PeriodStart,
			&subjects, &fun, &rep.WatchMinutes, &rep.GeneratedAt); err != nil {
			return nil, err
		}
		rep.SubjectStats = map[string]SubjectStat{}
		if err := json.Unmarshal([]byte(subjects), &rep.SubjectStats); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(fun), &rep.FunStats); err != nil {
			return nil, err
		}
		reports = append(reports, rep)
	}
	return reports, rows.Err()
}

func (r *repository) children(ctx context.Context) ([]childRow, error) {
	rows, err := r.pool.Query(ctx, `
		select c.id, coalesce(wr.timezone, 'Asia/Shanghai')
		from child_profile c
		left join watch_rule wr on wr.child_id = c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	children := make([]childRow, 0)
	for rows.Next() {
		var child childRow
		if err := rows.Scan(&child.ID, &child.Timezone); err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	return children, rows.Err()
}

func (r *repository) ownsChild(ctx context.Context, accountID, childID string) (bool, error) {
	var owns bool
	err := r.pool.QueryRow(ctx, `
		select exists (
			select 1 from child_profile c
			join family_member fm on fm.family_id = c.family_id
			where c.id = $1 and fm.account_id = $2
		)`, childID, accountID).Scan(&owns)
	return owns, err
}

type childRow struct {
	ID       string
	Timezone string
}

func isNoRows(err error) bool {
	return err == pgx.ErrNoRows
}

// subjectWeaknesses 返回最近 days 天各学科正确率（样本 >= minSample）。
func (r *repository) subjectWeaknesses(ctx context.Context, childID string, days, minSample int) ([]SubjectWeakness, error) {
	rows, err := r.pool.Query(ctx, `
		select q.subject_code, count(*)::int,
		       count(*) filter (where a.is_correct)::int
		from answer_log a
		join question q on q.id = a.question_id
		where a.child_id = $1 and a.answered_at >= now() - ($2 * interval '1 day')
		group by q.subject_code
		having count(*) >= $3
		order by (count(*) filter (where a.is_correct))::numeric / count(*) asc
		limit 3`, childID, days, minSample)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SubjectWeakness, 0)
	for rows.Next() {
		var item SubjectWeakness
		if err := rows.Scan(&item.Subject, &item.Answered, &item.Correct); err != nil {
			return nil, err
		}
		item.Accuracy = float64(item.Correct) / float64(item.Answered)
		items = append(items, item)
	}
	return items, rows.Err()
}

// recommendationBaseline 返回孩子学段、最近平均难度与整体正确率。
func (r *repository) recommendationBaseline(ctx context.Context, childID string) (string, int, float64, error) {
	var stage string
	if err := r.pool.QueryRow(ctx,
		`select stage_code from child_profile where id = $1`, childID).Scan(&stage); err != nil {
		return "", 0, 0, err
	}
	var avgDifficulty *float64
	var answered, correct int
	err := r.pool.QueryRow(ctx, `
		select avg(q.difficulty)::float8, count(*)::int,
		       count(*) filter (where a.is_correct)::int
		from answer_log a
		join question q on q.id = a.question_id
		where a.child_id = $1 and a.answered_at >= now() - interval '30 days'`,
		childID).Scan(&avgDifficulty, &answered, &correct)
	if err != nil {
		return "", 0, 0, err
	}
	difficulty := 2
	if avgDifficulty != nil {
		difficulty = int(math.Round(*avgDifficulty))
	}
	accuracy := 0.0
	if answered > 0 {
		accuracy = float64(correct) / float64(answered)
	}
	return stage, difficulty, accuracy, nil
}

// recommendationTopics 返回匹配学段/学科的在线专题。
func (r *repository) recommendationTopics(ctx context.Context, stage, subject string, limit int) ([]TopicSuggestion, error) {
	rows, err := r.pool.Query(ctx, `
		select tp.id, tp.name, count(*)::int
		from topic_pack tp
		join topic_question tq on tq.topic_id = tp.id
		join question q on q.id = tq.question_id and q.status = 'online'
		where tp.status = 'online'
		  and (jsonb_array_length(tp.stage_scope) = 0 or tp.stage_scope ? $1)
		  and ($2 = '' or jsonb_array_length(tp.subject_scope) = 0 or tp.subject_scope ? $2)
		group by tp.id, tp.name
		having count(*) >= 3
		order by count(*) desc
		limit $3`, stage, subject, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]TopicSuggestion, 0)
	for rows.Next() {
		var item TopicSuggestion
		var count int
		if err := rows.Scan(&item.TopicID, &item.Name, &count); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
