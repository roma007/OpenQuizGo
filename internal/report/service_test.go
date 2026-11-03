package report

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/roma007/OpenQuizGo/migrations"
)

func TestGenerateWeeklyReport(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareReportDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	accountID, childID := seedReportChild(t, pool)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	weekStart := time.Date(2026, 11, 2, 0, 0, 0, 0, loc) // Monday

	mathID := insertQuestion(t, pool, "math", "core")
	chineseID := insertQuestion(t, pool, "chinese", "core")
	funID := insertQuestion(t, pool, "fun", "fun")

	insertAnswer(t, pool, childID, mathID, weekStart.Add(10*time.Hour), true)
	insertAnswer(t, pool, childID, mathID, weekStart.AddDate(0, 0, 1).Add(10*time.Hour), false)
	insertAnswer(t, pool, childID, chineseID, weekStart.Add(11*time.Hour), true)
	insertAnswer(t, pool, childID, funID, weekStart.Add(12*time.Hour), true)
	insertAnswer(t, pool, childID, funID, weekStart.AddDate(0, 0, 1).Add(12*time.Hour), true)
	// 周期外数据不应计入
	insertAnswer(t, pool, childID, mathID, weekStart.AddDate(0, 0, -1), true)

	insertWatch(t, pool, childID, weekStart.Add(9*time.Hour), 600)
	insertWatch(t, pool, childID, weekStart.AddDate(0, 0, 1).Add(9*time.Hour), 300)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := New(pool, logger)

	rep, err := svc.Generate(ctx, childID, "week", weekStart)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if rep.SubjectStats["math"].Answered != 2 || rep.SubjectStats["math"].Correct != 1 {
		t.Fatalf("math stats: %+v", rep.SubjectStats["math"])
	}
	if rep.SubjectStats["math"].Accuracy != 0.5 {
		t.Fatalf("math accuracy = %v", rep.SubjectStats["math"].Accuracy)
	}
	if _, ok := rep.SubjectStats["fun"]; ok {
		t.Fatal("fun must not appear in subject_stats")
	}
	if rep.FunStats.Answered != 2 || rep.FunStats.Correct != 2 {
		t.Fatalf("fun stats: %+v", rep.FunStats)
	}
	if rep.WatchMinutes != 15 {
		t.Fatalf("watch minutes = %d, want 15", rep.WatchMinutes)
	}

	again, err := svc.Generate(ctx, childID, "week", weekStart)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if again.ID != rep.ID {
		t.Fatalf("idempotent generation changed id: %s vs %s", again.ID, rep.ID)
	}
	var count int
	if err := pool.QueryRow(ctx,
		`select count(*) from growth_report where child_id = $1 and period = 'week'`,
		childID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("report rows = %d, want 1", count)
	}

	list, err := svc.List(ctx, accountID, childID, "week", 8)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || !list[0].PeriodEnd.Equal(weekStart.AddDate(0, 0, 7)) {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestPeriodStart(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	monday := periodStart("week", time.Date(2026, 11, 4, 15, 0, 0, 0, loc), loc)
	if monday.Weekday() != time.Monday || monday.Day() != 2 {
		t.Fatalf("week start = %s", monday)
	}
	month := periodStart("month", time.Date(2026, 11, 4, 15, 0, 0, 0, loc), loc)
	if month.Day() != 1 || month.Month() != time.November {
		t.Fatalf("month start = %s", month)
	}
}

func prepareReportDB(t *testing.T, url string) {
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
		growth_report, answer_log, watch_log, watch_grant,
		quiz_session_item, quiz_session, watch_rule,
		child_profile, family_member, family, parent_account
		cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func seedReportChild(t *testing.T, pool *pgxpool.Pool) (accountID, childID string) {
	t.Helper()
	ctx := context.Background()
	var familyID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'hash-report')
		returning id`).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`insert into family (owner_id) values ($1) returning id`, accountID).Scan(&familyID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		insert into family_member (family_id, account_id, role, second_factor_verified)
		values ($1, $2, 'owner', true)`, familyID, accountID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		insert into child_profile (family_id, nickname, stage_code)
		values ($1, '报告孩子', 'low') returning id`, familyID).Scan(&childID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		insert into watch_rule (child_id, subject_weight, timezone)
		values ($1, '{"math":2,"fun":0.5}'::jsonb, 'Asia/Shanghai')`, childID); err != nil {
		t.Fatal(err)
	}
	return accountID, childID
}

func insertQuestion(t *testing.T, pool *pgxpool.Pool, subject, category string) string {
	t.Helper()
	alias := "null"
	if category == "fun" {
		alias = `'["同义词"]'::jsonb`
	}
	var id string
	err := pool.QueryRow(context.Background(), `
		insert into question (subject_code, category_code, stage_code, difficulty, type_code, stem,
			options, answer, answer_alias, status, version, schema_ver)
		values ($1, $2, 'low', 2, 'choice', '测试题',
		        '[{"id":"a","text":"1"},{"id":"b","text":"2"}]'::jsonb,
		        '["a"]'::jsonb, `+alias+`, 'online', 1, 1)
		returning id`, subject, category).Scan(&id)
	if err != nil {
		t.Fatalf("insert question: %v", err)
	}
	return id
}

func insertAnswer(t *testing.T, pool *pgxpool.Pool, childID, questionID string, at time.Time, correct bool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		insert into answer_log (child_id, question_id, device_id, is_correct, answered_at)
		values ($1, $2, 'tv-report', $3, $4)`, childID, questionID, correct, at); err != nil {
		t.Fatalf("insert answer: %v", err)
	}
}

func insertWatch(t *testing.T, pool *pgxpool.Pool, childID string, at time.Time, seconds int) {
	t.Helper()
	end := at.Add(time.Duration(seconds) * time.Second)
	if _, err := pool.Exec(context.Background(), `
		insert into watch_log (child_id, device_id, start_at, end_at, watched_seconds)
		values ($1, 'tv-report', $2, $3, $4)`,
		childID, at, end, seconds); err != nil {
		t.Fatalf("insert watch: %v", err)
	}
}
