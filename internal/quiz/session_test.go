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

	svc := New(pool, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
