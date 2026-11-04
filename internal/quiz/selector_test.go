package quiz

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/roma007/OpenQuizGo/migrations"
)

func TestTargetDifficulty(t *testing.T) {
	if got := targetDifficulty("low", "g2", nil); got != 1 {
		t.Fatalf("low/g2 = %d, want 1", got)
	}
	if got := targetDifficulty("high", "g8", nil); got != 4 {
		t.Fatalf("high/g8 = %d, want 4", got)
	}
	if got := targetDifficulty("mid", "", map[int]float64{4: 0.7}); got != 4 {
		t.Fatalf("stats target = %d, want 4", got)
	}
}

func TestSelectWithRealPool(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareQuizDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	childID := seedChild(t, pool)
	offlineID, answeredID := seedQuestions(t, pool, childID)

	svc := New(pool, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rule := Rule{
		UnlockMinutes: 20,
		PassThreshold: 3,
		SubjectWeight: map[string]float64{
			"math": 2, "chinese": 1, "science": 1, "fun": 0.5,
		},
		FunRatioCap: 2,
		FunEnabled:  true,
	}

	first, err := svc.Select(ctx, SelectRequest{ChildID: childID, Rule: rule, Seed: 42})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	second, err := svc.Select(ctx, SelectRequest{ChildID: childID, Rule: rule, Seed: 42})
	if err != nil {
		t.Fatalf("select again: %v", err)
	}
	if len(first.Selected) != len(second.Selected) {
		t.Fatalf("non-deterministic: %d vs %d", len(first.Selected), len(second.Selected))
	}
	for i := range first.Selected {
		if first.Selected[i].QuestionID != second.Selected[i].QuestionID {
			t.Fatal("same seed produced different question order")
		}
	}

	if first.Total < 3 || first.Total > 12 {
		t.Fatalf("total = %d, want 3..12", first.Total)
	}
	if first.MaxScore < rule.PassThreshold {
		t.Fatalf("max score = %.1f < threshold", first.MaxScore)
	}
	funCount := 0
	for _, item := range first.Selected {
		if item.Subject == "fun" {
			funCount++
		}
		if item.QuestionID == offlineID {
			t.Fatal("offline question was selected")
		}
		if item.QuestionID == answeredID {
			t.Fatal("recently answered question was selected")
		}
	}
	if funCount > rule.FunRatioCap {
		t.Fatalf("fun count = %d, cap = %d", funCount, rule.FunRatioCap)
	}

	for seed := int64(1); seed <= 20; seed++ {
		result, err := svc.Select(ctx, SelectRequest{ChildID: childID, Rule: rule, Seed: seed})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		for _, item := range result.Selected {
			if item.QuestionID == offlineID {
				t.Fatalf("seed %d selected offline question", seed)
			}
			if item.QuestionID == answeredID {
				t.Fatalf("seed %d selected recently answered question", seed)
			}
			if item.Repeated {
				t.Fatalf("seed %d unexpectedly used repeated question", seed)
			}
		}
	}

	emptyRule := Rule{
		UnlockMinutes: 20,
		PassThreshold: 3,
		SubjectWeight: map[string]float64{"english": 2},
		DailyLimit:    60,
		Timezone:      "Asia/Shanghai",
	}
	_, err = svc.Select(ctx, SelectRequest{Stage: "low", Rule: emptyRule, Seed: 1})
	quizErr, ok := err.(*Error)
	if !ok || quizErr.Code != "INSUFFICIENT_POOL" {
		t.Fatalf("expected INSUFFICIENT_POOL, got %v", err)
	}
}

func prepareQuizDB(t *testing.T, url string) {
	t.Helper()
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`truncate table
		answer_log, quiz_session_item, quiz_session, watch_log, watch_grant,
		child_profile, family_member, family, parent_account
		cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func seedChild(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var accountID, familyID, childID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'hash-quiz-test')
		returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`insert into family (owner_id) values ($1) returning id`, accountID).Scan(&familyID); err != nil {
		t.Fatalf("family: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into family_member (family_id, account_id, role, second_factor_verified)
		values ($1, $2, 'owner', true)`, familyID, accountID); err != nil {
		t.Fatalf("member: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		insert into child_profile (family_id, nickname, stage_code, grade_tag)
		values ($1, '测试孩子', 'low', 'g2') returning id`, familyID).Scan(&childID); err != nil {
		t.Fatalf("child: %v", err)
	}
	return childID
}

func seedQuestions(t *testing.T, pool *pgxpool.Pool, childID string) (offlineID, answeredID string) {
	t.Helper()
	ctx := context.Background()
	specs := []struct {
		subject  string
		category string
	}{
		{"math", "core"},
		{"chinese", "core"},
		{"science", "literacy"},
		{"fun", "fun"},
	}
	for _, spec := range specs {
		for difficulty := 1; difficulty <= 5; difficulty++ {
			for i := 0; i < 10; i++ {
				alias := "null"
				if spec.category == "fun" {
					alias = `'["同义词"]'::jsonb`
				}
				if _, err := pool.Exec(ctx, `
					insert into question (
						subject_code, category_code, stage_code, difficulty, type_code, stem,
						options, answer, answer_alias, status, version, schema_ver)
					values ($1, $2, 'low', $3, 'choice', $4,
					        '[{"id":"a","text":"1"},{"id":"b","text":"2"}]'::jsonb,
					        '["a"]'::jsonb, `+alias+`, 'online', 1, 1)`,
					spec.subject, spec.category, difficulty,
					spec.subject+"-"+string(rune('0'+difficulty))+"-"+string(rune('a'+i))); err != nil {
					t.Fatalf("insert question: %v", err)
				}
			}
		}
	}

	if err := pool.QueryRow(ctx, `
		insert into question (
			subject_code, category_code, stage_code, difficulty, type_code, stem,
			options, answer, status, version, schema_ver)
		values ('math', 'core', 'low', 2, 'choice', '已下线题目',
		        '[{"id":"a","text":"1"},{"id":"b","text":"2"}]'::jsonb,
		        '["a"]'::jsonb, 'offline', 1, 1)
		returning id`).Scan(&offlineID); err != nil {
		t.Fatalf("insert offline: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		select id from question
		where subject_code = 'math' and status = 'online'
		order by id limit 1`).Scan(&answeredID); err != nil {
		t.Fatalf("pick answered question: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into answer_log (child_id, question_id, device_id, is_correct, answered_at)
		values ($1, $2, 'tv-test', true, now())`, childID, answeredID); err != nil {
		t.Fatalf("insert answer log: %v", err)
	}
	return offlineID, answeredID
}

func TestSelectPrefersActiveTopicPack(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareQuizDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `truncate table topic_question, topic_pack, question cascade`); err != nil {
		t.Fatalf("truncate topic: %v", err)
	}

	childID := seedChild(t, pool)
	questionIDs := seedTopicQuestions(t, pool, 5)
	packID := seedTopicPack(t, pool)
	for position, id := range questionIDs {
		if _, err := pool.Exec(ctx,
			`insert into topic_question (topic_id, question_id, position) values ($1, $2, $3)`,
			packID, id, position); err != nil {
			t.Fatalf("link question: %v", err)
		}
	}

	svc := New(pool, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rule := Rule{
		UnlockMinutes: 20,
		PassThreshold: 3,
		SubjectWeight: map[string]float64{"math": 2, "chinese": 1},
		DailyLimit:    60,
		Timezone:      "Asia/Shanghai",
	}

	selectTopic := func(seed int64) Result {
		t.Helper()
		result, err := svc.Select(ctx, SelectRequest{ChildID: childID, Rule: rule, Seed: seed})
		if err != nil {
			t.Fatalf("select: %v", err)
		}
		return result
	}

	result := selectTopic(7)
	if result.TopicID != packID || result.TopicName != "寒假数学专题" {
		t.Fatalf("topic = %q/%q, want %q", result.TopicID, result.TopicName, packID)
	}
	if result.Total != 5 {
		t.Fatalf("total = %d, want 5", result.Total)
	}
	inPack := map[string]bool{}
	for _, id := range questionIDs {
		inPack[id] = true
	}
	for _, item := range result.Selected {
		if !inPack[item.QuestionID] {
			t.Fatalf("selected question %s is not in topic pack", item.QuestionID)
		}
	}
	if !hasWarning(result.Warnings, "topic_mode:寒假数学专题") {
		t.Fatalf("warnings = %v, want topic_mode", result.Warnings)
	}

	// 下线后回退普通组卷。
	if _, err := pool.Exec(ctx, `update topic_pack set status = 'offline' where id = $1`, packID); err != nil {
		t.Fatalf("offline: %v", err)
	}
	if got := selectTopic(8); got.TopicID != "" {
		t.Fatalf("offline pack still used: %q", got.TopicID)
	}

	// 过期后回退。
	if _, err := pool.Exec(ctx, `update topic_pack set status = 'online', end_at = now() - interval '1 hour' where id = $1`, packID); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if got := selectTopic(9); got.TopicID != "" {
		t.Fatalf("expired pack still used: %q", got.TopicID)
	}

	// 学科不匹配时回退。
	if _, err := pool.Exec(ctx, `
		update topic_pack set end_at = now() + interval '1 day', subject_scope = '["english"]'::jsonb
		where id = $1`, packID); err != nil {
		t.Fatalf("subject scope: %v", err)
	}
	if got := selectTopic(10); got.TopicID != "" {
		t.Fatalf("subject-mismatched pack still used: %q", got.TopicID)
	}

	// 学段不匹配时回退。
	if _, err := pool.Exec(ctx, `
		update topic_pack set subject_scope = '["math"]'::jsonb, stage_scope = '["high"]'::jsonb
		where id = $1`, packID); err != nil {
		t.Fatalf("stage scope: %v", err)
	}
	if got := selectTopic(11); got.TopicID != "" {
		t.Fatalf("stage-mismatched pack still used: %q", got.TopicID)
	}

	// 题目不足 3 道时回退。
	if _, err := pool.Exec(ctx, `update topic_pack set stage_scope = '["low"]'::jsonb where id = $1`, packID); err != nil {
		t.Fatalf("restore stage: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		delete from topic_question where topic_id = $1 and position >= 2`, packID); err != nil {
		t.Fatalf("shrink topic: %v", err)
	}
	if got := selectTopic(12); got.TopicID != "" {
		t.Fatalf("short pack still used: %q", got.TopicID)
	}
}

func seedTopicQuestions(t *testing.T, pool *pgxpool.Pool, n int) []string {
	t.Helper()
	ctx := context.Background()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		var id string
		err := pool.QueryRow(ctx, `
			insert into question (
				subject_code, category_code, stage_code, difficulty, type_code, stem,
				options, answer, status, version, schema_ver)
			values ('math', 'core', 'low', 2, 'choice', $1,
			        '[{"id":"a","text":"1"},{"id":"b","text":"2"}]'::jsonb,
			        '["a"]'::jsonb, 'online', 1, 1)
			returning id`, "专题题 "+string(rune('A'+i))).Scan(&id)
		if err != nil {
			t.Fatalf("insert topic question: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func seedTopicPack(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		insert into topic_pack (
			name, type, subject_scope, stage_scope, is_paid, status, start_at, end_at)
		values ('寒假数学专题', 'vacation', '["math"]'::jsonb, '["low"]'::jsonb,
		        false, 'online', now() - interval '1 hour', now() + interval '1 day')
		returning id`).Scan(&id)
	if err != nil {
		t.Fatalf("insert topic pack: %v", err)
	}
	return id
}

func hasWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if warning == want {
			return true
		}
	}
	return false
}

func TestPaidTopicRequiresEntitlement(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareQuizDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `truncate table
		topic_question, topic_pack, question,
		membership_order, account_membership, membership_plan cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	childID := seedChild(t, pool)
	questionIDs := seedTopicQuestions(t, pool, 5)
	packID := seedTopicPack(t, pool)
	if _, err := pool.Exec(ctx, `update topic_pack set is_paid = true where id = $1`, packID); err != nil {
		t.Fatalf("paid pack: %v", err)
	}
	for position, id := range questionIDs {
		if _, err := pool.Exec(ctx,
			`insert into topic_question (topic_id, question_id, position) values ($1, $2, $3)`,
			packID, id, position); err != nil {
			t.Fatalf("link: %v", err)
		}
	}

	var ownerID, familyID string
	if err := pool.QueryRow(ctx, `
		select f.owner_id, f.id from child_profile cp
		join family f on f.id = cp.family_id where cp.id = $1`, childID).
		Scan(&ownerID, &familyID); err != nil {
		t.Fatalf("owner: %v", err)
	}

	svc := New(pool, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rule := Rule{
		UnlockMinutes: 20,
		PassThreshold: 3,
		SubjectWeight: map[string]float64{"math": 2},
		DailyLimit:    60,
		Timezone:      "Asia/Shanghai",
	}

	if result, err := svc.Select(ctx, SelectRequest{ChildID: childID, Rule: rule, Seed: 21}); err != nil {
		t.Fatalf("select without membership: %v", err)
	} else if result.TopicID != "" {
		t.Fatalf("paid topic used without entitlement: %q", result.TopicID)
	}

	var planID string
	if err := pool.QueryRow(ctx, `
		insert into membership_plan (code, name, duration_days, price_cents, features, entitlement_codes, status, sort)
		values ('test-topic', '专题权益卡', 30, 0, '[]'::jsonb, '["topic_pack"]'::jsonb, 'online', 1)
		returning id`).Scan(&planID); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into account_membership (account_id, plan_id, plan_name, status, starts_at, expires_at)
		values ($1, $2, '专题权益卡', 'active', now(), now() + interval '30 days')`,
		ownerID, planID); err != nil {
		t.Fatalf("membership: %v", err)
	}

	result, err := svc.Select(ctx, SelectRequest{ChildID: childID, Rule: rule, Seed: 22})
	if err != nil {
		t.Fatalf("select with membership: %v", err)
	}
	if result.TopicID != packID {
		t.Fatalf("paid topic not used with entitlement: %q", result.TopicID)
	}

	if _, err := pool.Exec(ctx, `
		update account_membership set expires_at = now() - interval '1 hour' where account_id = $1`,
		ownerID); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if result, err := svc.Select(ctx, SelectRequest{ChildID: childID, Rule: rule, Seed: 23}); err != nil {
		t.Fatalf("select expired: %v", err)
	} else if result.TopicID != "" {
		t.Fatalf("paid topic used after expiry: %q", result.TopicID)
	}
}
