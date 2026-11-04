package quiz

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	pool *pgxpool.Pool
}

func newRepository(pool *pgxpool.Pool) *repository {
	return &repository{pool: pool}
}

// Candidate 是候选题目。
type Candidate struct {
	ID         string
	Subject    string
	Stage      string
	Type       string
	Difficulty int
}

// pool 返回某学科在给定学段的在线题目；excludeRecent 为 true 时排除孩子近 30 天做过的题。
func (r *repository) candidates(ctx context.Context, childID, subject string, stages []string, excludeRecent bool, limit int) ([]Candidate, error) {
	var childArg any
	if childID != "" && excludeRecent {
		childArg = childID
	}

	const query = `
		select q.id, q.subject_code, q.stage_code, q.type_code, q.difficulty
		from question q
		where q.status = 'online'
		  and q.subject_code = $1
		  and q.stage_code = any($2)
		  and ($3::uuid is null or not exists (
		      select 1 from answer_log a
		      where a.child_id = $3::uuid
		        and a.question_id = q.id
		        and a.answered_at > now() - interval '30 days'))
		order by q.id
		limit $4`

	rows, err := r.pool.Query(ctx, query, subject, stages, childArg, limit)
	if err != nil {
		return nil, fmt.Errorf("query pool: %w", err)
	}
	defer rows.Close()

	items := make([]Candidate, 0, limit)
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ID, &c.Subject, &c.Stage, &c.Type, &c.Difficulty); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// topicPack 是参与组卷的专题包。
type topicPack struct {
	ID   string
	Name string
}

// activeTopicPack 返回匹配学段/学科且在有效期内的在线专题（题目数 >= 3）。
func (r *repository) activeTopicPack(ctx context.Context, stage string, subjects, stages []string) (topicPack, bool, error) {
	var pack topicPack
	err := r.pool.QueryRow(ctx, `
		select tp.id, tp.name
		from topic_pack tp
		join topic_question tq on tq.topic_id = tp.id
		join question q on q.id = tq.question_id
		where tp.status = 'online'
		  and (tp.start_at is null or tp.start_at <= now())
		  and (tp.end_at is null or tp.end_at >= now())
		  and (jsonb_array_length(tp.stage_scope) = 0 or tp.stage_scope ? $1)
		  and (jsonb_array_length(tp.subject_scope) = 0
		       or exists (
		           select 1 from jsonb_array_elements_text(tp.subject_scope) s
		           where s = any($2)))
		  and q.status = 'online'
		  and q.stage_code = any($3)
		group by tp.id, tp.name, tp.updated_at
		having count(*) >= 3
		order by tp.updated_at desc
		limit 1`, stage, subjects, stages).Scan(&pack.ID, &pack.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return topicPack{}, false, nil
	}
	if err != nil {
		return topicPack{}, false, err
	}
	return pack, true, nil
}

// topicCandidates 返回专题内匹配学段的在线题目；excludeRecent 为 true 时排除孩子近 30 天做过的题。
func (r *repository) topicCandidates(ctx context.Context, childID, topicID string, stages []string, excludeRecent bool, limit int) ([]Candidate, error) {
	var childArg any
	if childID != "" && excludeRecent {
		childArg = childID
	}

	rows, err := r.pool.Query(ctx, `
		select q.id, q.subject_code, q.stage_code, q.type_code, q.difficulty
		from topic_question tq
		join question q on q.id = tq.question_id
		where tq.topic_id = $1
		  and q.status = 'online'
		  and q.stage_code = any($2)
		  and ($3::uuid is null or not exists (
		      select 1 from answer_log a
		      where a.child_id = $3::uuid
		        and a.question_id = q.id
		        and a.answered_at > now() - interval '30 days'))
		order by tq.position, q.id
		limit $4`, topicID, stages, childArg, limit)
	if err != nil {
		return nil, fmt.Errorf("query topic pool: %w", err)
	}
	defer rows.Close()

	items := make([]Candidate, 0, limit)
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ID, &c.Subject, &c.Stage, &c.Type, &c.Difficulty); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// childStage 读取孩子学段；childID 为空时返回空。
func (r *repository) childStage(ctx context.Context, childID string) (stage, grade string, err error) {
	if childID == "" {
		return "", "", nil
	}
	err = r.pool.QueryRow(ctx, `
		select stage_code, coalesce(grade_tag, '')
		from child_profile where id = $1`, childID).Scan(&stage, &grade)
	return stage, grade, err
}

// difficultyStats 返回孩子近 30 天各难度的正确率。
func (r *repository) difficultyStats(ctx context.Context, childID string) (map[int]float64, error) {
	if childID == "" {
		return map[int]float64{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		select difficulty, count(*) as attempts,
		       sum(case when a.is_correct then 1 else 0 end)::float / count(*) as rate
		from answer_log a
		join question q on q.id = a.question_id
		where a.child_id = $1 and a.answered_at > now() - interval '30 days'
		group by difficulty`, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := map[int]float64{}
	for rows.Next() {
		var difficulty, attempts int
		var rate float64
		if err := rows.Scan(&difficulty, &attempts, &rate); err != nil {
			return nil, err
		}
		if attempts >= 5 {
			stats[difficulty] = rate
		}
	}
	return stats, rows.Err()
}
