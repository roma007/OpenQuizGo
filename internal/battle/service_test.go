package battle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/roma007/OpenQuizGo/migrations"
)

func TestBattleWorkflow(t *testing.T) {
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

	accountID, childID := seedFamily(t, pool)
	questionIDs := seedQuestions(t, pool, 5)
	svc := New(pool)

	state, err := svc.Create(ctx, accountID, childID, 5)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if state.Status != "child_turn" || state.QuestionCount != 5 || len(state.Questions) != 5 {
		t.Fatalf("unexpected create state: %+v", state)
	}
	if len(state.Answers) != 0 || state.Result != nil {
		t.Fatal("in-progress battle leaked answers or result")
	}

	childAnswers := make([]Submission, 0, len(questionIDs))
	for _, id := range questionIDs {
		childAnswers = append(childAnswers, Submission{QuestionID: id, Answer: json.RawMessage(`["a"]`)})
	}
	state, err = svc.Submit(ctx, childID, state.ID, "child", childAnswers)
	if err != nil {
		t.Fatalf("child submit: %v", err)
	}
	if state.Status != "parent_turn" || state.ChildScore != 5 || len(state.Answers) != 0 {
		t.Fatalf("unexpected child state: %+v", state)
	}

	if _, err := svc.Submit(ctx, childID, state.ID, "child", childAnswers); err == nil || !isCode(err, "BATTLE_TURN_INVALID") {
		t.Fatalf("duplicate child submit = %v, want BATTLE_TURN_INVALID", err)
	}
	if _, err := svc.Submit(ctx, childID, state.ID, "parent", childAnswers[:2]); err == nil || !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("short answers = %v, want VALIDATION_ERROR", err)
	}

	parentAnswers := make([]Submission, 0, len(questionIDs))
	for i, id := range questionIDs {
		answer := json.RawMessage(`["a"]`)
		if i%2 == 1 {
			answer = json.RawMessage(`["b"]`)
		}
		parentAnswers = append(parentAnswers, Submission{QuestionID: id, Answer: answer})
	}
	state, err = svc.Submit(ctx, childID, state.ID, "parent", parentAnswers)
	if err != nil {
		t.Fatalf("parent submit: %v", err)
	}
	if state.Status != "finished" || state.ChildScore != 5 || state.ParentScore != 3 {
		t.Fatalf("unexpected finished state: %+v", state)
	}
	if state.Result == nil || state.Result.Winner != "child" {
		t.Fatalf("unexpected result: %+v", state.Result)
	}
	if len(state.Answers) != 10 {
		t.Fatalf("answers = %d, want 10", len(state.Answers))
	}

	loaded, err := svc.Get(ctx, childID, state.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.Result == nil || loaded.Result.Winner != "child" || len(loaded.Answers) != 10 {
		t.Fatalf("unexpected loaded state: %+v", loaded)
	}
	if _, err := svc.Submit(ctx, childID, state.ID, "parent", parentAnswers); err == nil || !isCode(err, "BATTLE_TURN_INVALID") {
		t.Fatalf("submit after finish = %v, want BATTLE_TURN_INVALID", err)
	}

	// 非本家庭账号不能创建对局。
	var outsider string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'outsider')
		returning id`).Scan(&outsider); err != nil {
		t.Fatalf("outsider: %v", err)
	}
	if _, err := svc.Create(ctx, outsider, childID, 3); err == nil || !isCode(err, "CHILD_NOT_FOUND") {
		t.Fatalf("outsider create = %v, want CHILD_NOT_FOUND", err)
	}

	// 题库不足时拒绝创建。
	if _, err := pool.Exec(ctx, `delete from family_battle`); err != nil {
		t.Fatalf("clear battles: %v", err)
	}
	if _, err := pool.Exec(ctx, `delete from question`); err != nil {
		t.Fatalf("clear questions: %v", err)
	}
	if _, err := svc.Create(ctx, accountID, childID, 5); err == nil || !isCode(err, "QUESTION_POOL_EMPTY") {
		t.Fatalf("empty pool create = %v, want QUESTION_POOL_EMPTY", err)
	}
}

func isCode(err error, code string) bool {
	var e apiError
	return errors.As(err, &e) && e.Code == code
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
	if _, err := db.Exec(`truncate table
		family_battle, family_battle_answer, question,
		child_profile, family_member, family, parent_account
		cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func seedFamily(t *testing.T, pool *pgxpool.Pool) (accountID, childID string) {
	t.Helper()
	ctx := context.Background()
	var familyID string
	if err := pool.QueryRow(ctx, `
		insert into parent_account (phone_cipher, phone_hash) values ('cipher', 'battle-owner')
		returning id`).Scan(&accountID); err != nil {
		t.Fatalf("account: %v", err)
	}
	if err := pool.QueryRow(ctx, `insert into family (owner_id) values ($1) returning id`,
		accountID).Scan(&familyID); err != nil {
		t.Fatalf("family: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into family_member (family_id, account_id, role, second_factor_verified)
		values ($1, $2, 'owner', true)`, familyID, accountID); err != nil {
		t.Fatalf("member: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		insert into child_profile (family_id, nickname, stage_code, grade_tag)
		values ($1, '对战孩子', 'low', 'g2') returning id`, familyID).Scan(&childID); err != nil {
		t.Fatalf("child: %v", err)
	}
	return accountID, childID
}

func seedQuestions(t *testing.T, pool *pgxpool.Pool, n int) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		var id string
		err := pool.QueryRow(context.Background(), `
			insert into question (
				subject_code, category_code, stage_code, difficulty, type_code, stem,
				options, answer, explain, status, version, schema_ver)
			values ('math', 'core', 'low', 2, 'choice', $1,
			        '[{"id":"a","text":"1"},{"id":"b","text":"2"}]'::jsonb,
			        '["a"]'::jsonb, '解析', 'online', 1, 1)
			returning id`, "对战题").Scan(&id)
		if err != nil {
			t.Fatalf("question: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}
