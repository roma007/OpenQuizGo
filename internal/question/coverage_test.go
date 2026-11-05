package question

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/roma007/OpenQuizGo/migrations"
)

func TestTopicCoverageWarnings(t *testing.T) {
	cells := []TopicCoverageCell{
		{TopicID: "t1", TopicName: "寒假数学", Status: "online", Count: 12},
		{TopicID: "t1", TopicName: "寒假数学", Status: "draft", Count: 5},
		{TopicID: "t2", TopicName: "英语阅读", Status: "online", Count: 60},
		{TopicID: "", TopicName: topicCoverageUnassigned, Status: "online", Count: 3},
	}

	warnings := topicCoverageWarnings(cells, 50)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want 1 item", warnings)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "寒假数学") || !strings.Contains(joined, "上线题量 12") {
		t.Fatalf("missing topic warning: %v", warnings)
	}
	if strings.Contains(joined, topicCoverageUnassigned) {
		t.Fatalf("unassigned should not warn: %v", warnings)
	}
	if strings.Contains(joined, "英语阅读") {
		t.Fatalf("topic above threshold should not warn: %v", warnings)
	}

	if got := topicCoverageWarnings(cells, 1); len(got) != 0 {
		t.Fatalf("threshold 1 warnings = %v, want none", got)
	}
}

func TestTopicCoverageAggregation(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	prepareCoverageDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	topicA := insertTopic(t, pool, "寒假数学")
	topicB := insertTopic(t, pool, "英语阅读")
	q1 := insertCoverageQuestion(t, pool, "多专题题", "online")
	q2 := insertCoverageQuestion(t, pool, "单专题题", "online")
	_ = insertCoverageQuestion(t, pool, "未分专题题", "draft")

	linkTopic(t, pool, topicA, q1)
	linkTopic(t, pool, topicB, q1)
	linkTopic(t, pool, topicA, q2)

	repo := newRepository(pool)
	cells, total, err := repo.TopicCoverage(ctx)
	if err != nil {
		t.Fatalf("topic coverage: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3 distinct questions", total)
	}

	counts := map[string]int{}
	sum := 0
	for _, cell := range cells {
		counts[cell.TopicID+"|"+cell.Status] += cell.Count
		sum += cell.Count
	}
	if counts[topicA+"|online"] != 2 {
		t.Fatalf("topic A online = %d, want 2 (multi-topic counted per relation): %+v",
			counts[topicA+"|online"], cells)
	}
	if counts[topicB+"|online"] != 1 {
		t.Fatalf("topic B online = %d, want 1: %+v", counts[topicB+"|online"], cells)
	}
	if counts["|draft"] != 1 {
		t.Fatalf("unassigned draft = %d, want 1: %+v", counts["|draft"], cells)
	}
	if sum != 4 {
		t.Fatalf("cell count sum = %d, want 4 (multi-topic duplicates): %+v", sum, cells)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &Service{repo: repo, logger: logger}
	result, err := svc.TopicCoverage(ctx, 50)
	if err != nil {
		t.Fatalf("service topic coverage: %v", err)
	}
	joined := strings.Join(result.Warnings, "\n")
	if !strings.Contains(joined, "寒假数学") || !strings.Contains(joined, "英语阅读") {
		t.Fatalf("warnings should include both low topics: %v", result.Warnings)
	}
	if strings.Contains(joined, topicCoverageUnassigned) {
		t.Fatalf("unassigned has no online question, should not warn: %v", result.Warnings)
	}
}

func prepareCoverageDB(t *testing.T, url string) {
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
	if _, err := db.Exec(`truncate table question, topic_pack cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func insertTopic(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		insert into topic_pack (name, type, status)
		values ($1, 'holiday', 'online')
		returning id::text`, name).Scan(&id); err != nil {
		t.Fatalf("insert topic %s: %v", name, err)
	}
	return id
}

func insertCoverageQuestion(t *testing.T, pool *pgxpool.Pool, stem, status string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		insert into question (
			subject_code, category_code, stage_code, difficulty, type_code, stem, answer, status)
		values ('math', 'core', 'low', 2, 'choice', $1, '["a"]'::jsonb, $2)
		returning id::text`, stem, status).Scan(&id); err != nil {
		t.Fatalf("insert question %s: %v", stem, err)
	}
	return id
}

func linkTopic(t *testing.T, pool *pgxpool.Pool, topicID, questionID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		insert into topic_question (topic_id, question_id) values ($1, $2)`,
		topicID, questionID); err != nil {
		t.Fatalf("link topic %s question %s: %v", topicID, questionID, err)
	}
}
