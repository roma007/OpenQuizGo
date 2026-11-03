package quiz

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/watch"
)

func TestSessionFlow(t *testing.T) {
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
	seedQuestions(t, pool, childID)

	svc := New(pool, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	principal := identity.DevicePrincipal{DeviceID: "tv-session", ChildID: childID}

	session, err := svc.CreateSession(ctx, principal)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if len(session.Questions) != 5 {
		t.Fatalf("questions = %d, want 5", len(session.Questions))
	}

	_, err = svc.CreateSession(ctx, principal)
	conflict, ok := err.(*Error)
	if !ok || conflict.Code != "QUIZ_SESSION_ACTIVE" || conflict.SessionID != session.SessionID {
		t.Fatalf("expected active session conflict, got %v", err)
	}

	wrong, err := svc.SubmitAnswer(ctx, principal, session.SessionID, AnswerSubmission{
		Token:  session.Questions[0].Token,
		Answer: json.RawMessage(`["b"]`),
	})
	if err != nil {
		t.Fatalf("submit wrong: %v", err)
	}
	if wrong.Correct {
		t.Fatal("expected wrong answer")
	}
	if !wrong.Suspicious {
		t.Fatal("fast answer should be marked suspicious")
	}

	correct, err := svc.SubmitAnswer(ctx, principal, session.SessionID, AnswerSubmission{
		Token:  session.Questions[1].Token,
		Answer: json.RawMessage(`["a"]`),
	})
	if err != nil {
		t.Fatalf("submit correct: %v", err)
	}
	if !correct.Correct || correct.Score <= 0 {
		t.Fatalf("expected correct answer with score, got %+v", correct)
	}

	_, err = svc.SubmitAnswer(ctx, principal, session.SessionID, AnswerSubmission{
		Token:  session.Questions[1].Token,
		Answer: json.RawMessage(`["a"]`),
	})
	reused, ok := err.(*Error)
	if !ok || reused.Code != "QUIZ_TOKEN_USED" {
		t.Fatalf("expected token reuse error, got %v", err)
	}

	for _, question := range session.Questions[2:] {
		if _, err := svc.SubmitAnswer(ctx, principal, session.SessionID, AnswerSubmission{
			Token:  question.Token,
			Answer: json.RawMessage(`["a"]`),
		}); err != nil {
			t.Fatalf("submit remaining: %v", err)
		}
	}

	summary, err := svc.FinishSession(ctx, principal, session.SessionID)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if summary.State != "passed" || summary.Score < summary.Threshold {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	recovered, err := svc.GetSession(ctx, principal, session.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if recovered.AnsweredCount != len(session.Questions) {
		t.Fatalf("answered = %d, want %d", recovered.AnsweredCount, len(session.Questions))
	}

	second, err := svc.CreateSession(ctx, principal)
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`update quiz_session set expires_at = now() - interval '1 minute' where id = $1`,
		second.SessionID); err != nil {
		t.Fatalf("expire session: %v", err)
	}
	_, err = svc.SubmitAnswer(ctx, principal, second.SessionID, AnswerSubmission{
		Token:  second.Questions[0].Token,
		Answer: json.RawMessage(`["a"]`),
	})
	expired, ok := err.(*Error)
	if !ok || expired.Code != "QUIZ_SESSION_EXPIRED" {
		t.Fatalf("expected expired error, got %v", err)
	}
}

func TestSessionIssuesGrant(t *testing.T) {
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
	seedQuestions(t, pool, childID)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	watchSvc := watch.New(pool, nil, "test-secret-0123456789-0123456789", logger)
	svc := New(pool, nil, nil, watchSvc, logger)
	principal := identity.DevicePrincipal{DeviceID: "tv-grant", ChildID: childID}

	session, err := svc.CreateSession(ctx, principal)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	for _, question := range session.Questions {
		if _, err := svc.SubmitAnswer(ctx, principal, session.SessionID, AnswerSubmission{
			Token:  question.Token,
			Answer: json.RawMessage(`["a"]`),
		}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	summary, err := svc.FinishSession(ctx, principal, session.SessionID)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if summary.State != "passed" || summary.Grant == nil {
		t.Fatalf("expected grant, got %+v", summary)
	}
	if summary.Grant.Minutes != 20 || summary.Grant.LeaseToken == "" {
		t.Fatalf("unexpected grant: %+v", summary.Grant)
	}

	again, err := svc.FinishSession(ctx, principal, session.SessionID)
	if err != nil {
		t.Fatalf("finish again: %v", err)
	}
	if again.Grant == nil || again.Grant.GrantID != summary.Grant.GrantID {
		t.Fatalf("finish should be idempotent: %+v", again.Grant)
	}

	if _, err := pool.Exec(ctx, `
		update watch_grant set last_heartbeat_at = now() - make_interval(secs => 30) where id = $1`,
		summary.Grant.GrantID); err != nil {
		t.Fatal(err)
	}
	beat, err := watchSvc.Heartbeat(ctx, principal, summary.Grant.GrantID, watch.HeartbeatRequest{})
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if beat.ConsumedSeconds < 29 || beat.ConsumedSeconds > 31 {
		t.Fatalf("consumed = %d, want ~30", beat.ConsumedSeconds)
	}
}
