package topic_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/roma007/OpenQuizGo/internal/topic"
	"github.com/roma007/OpenQuizGo/migrations"
)

func TestTopicPackWorkflow(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	prepareDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	questionIDs := seedQuestions(t, pool, 3)
	svc := topic.New(pool)

	created, err := svc.Create(ctx, topic.Input{
		Name:         "寒假专题",
		Type:         "vacation",
		SubjectScope: []string{"math"},
		StageScope:   []string{"low"},
		IsPaid:       false,
		Status:       "draft",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Name != "寒假专题" || created.Status != "draft" || created.QuestionCount != 0 {
		t.Fatalf("unexpected created pack: %+v", created)
	}

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("unexpected list: %+v", list)
	}

	// 选题：重复 id 去重，顺序按传入顺序。
	withQuestions, err := svc.SetQuestions(ctx, created.ID, []string{
		questionIDs[2], questionIDs[1], questionIDs[2], questionIDs[0],
	})
	if err != nil {
		t.Fatalf("set questions: %v", err)
	}
	if withQuestions.QuestionCount != 3 {
		t.Fatalf("question count = %d, want 3", withQuestions.QuestionCount)
	}
	wantOrder := []string{questionIDs[2], questionIDs[1], questionIDs[0]}
	for i, id := range wantOrder {
		if withQuestions.QuestionIDs[i] != id {
			t.Fatalf("question order = %v, want %v", withQuestions.QuestionIDs, wantOrder)
		}
	}

	updated, err := svc.Update(ctx, created.ID, topic.Input{
		Name:         "寒假专题（上线）",
		Type:         "vacation",
		SubjectScope: []string{"math", "chinese"},
		StageScope:   []string{"low"},
		IsPaid:       true,
		Status:       "online",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "寒假专题（上线）" || updated.Status != "online" || !updated.IsPaid {
		t.Fatalf("unexpected updated pack: %+v", updated)
	}
	if updated.QuestionCount != 3 {
		t.Fatalf("update lost questions: %+v", updated)
	}

	// 全量覆盖为空即清空题目。
	cleared, err := svc.SetQuestions(ctx, created.ID, nil)
	if err != nil {
		t.Fatalf("clear questions: %v", err)
	}
	if cleared.QuestionCount != 0 || len(cleared.QuestionIDs) != 0 {
		t.Fatalf("expected empty questions, got %+v", cleared)
	}

	if _, err := svc.SetQuestions(ctx, created.ID, []string{"00000000-0000-0000-0000-000000000000"}); err == nil {
		t.Fatal("expected error for unknown question id")
	}

	if _, err := svc.Create(ctx, topic.Input{Name: "", Type: "vacation", Status: "draft"}); err == nil {
		t.Fatal("expected validation error for empty name")
	}
	if _, err := svc.Create(ctx, topic.Input{Name: "x", Type: "unknown", Status: "draft"}); err == nil {
		t.Fatal("expected validation error for unknown type")
	}

	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get(ctx, created.ID); err == nil {
		t.Fatal("expected not found after delete")
	}
	if err := svc.Delete(ctx, created.ID); err == nil {
		t.Fatal("expected not found deleting twice")
	}
}

func prepareDB(t *testing.T, url string) {
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
	if _, err := db.Exec(`truncate table topic_question, topic_pack, question cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func seedQuestions(t *testing.T, pool *pgxpool.Pool, n int) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		var id string
		err := pool.QueryRow(context.Background(), `
			insert into question (subject_code, category_code, stage_code, difficulty,
			                      type_code, stem, options, answer, status)
			values ('math', 'core', 'low', 1, 'choice', $1,
			        '[{"id":"a","text":"1"}]'::jsonb, '["a"]'::jsonb, 'online')
			returning id`, "测试题").Scan(&id)
		if err != nil {
			t.Fatalf("seed question: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}
