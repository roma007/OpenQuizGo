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

	svc := New(pool, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
