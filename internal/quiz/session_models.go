package quiz

import (
	"encoding/json"
	"time"

	"github.com/roma007/OpenQuizGo/internal/watch"
)

const sessionTTL = 10 * time.Minute

type ruleSnapshot struct {
	Version int  `json:"version"`
	Rule    Rule `json:"rule"`
}

type itemSnapshot struct {
	Subject       string          `json:"subject"`
	Type          string          `json:"type"`
	Answer        json.RawMessage `json:"answer"`
	Alias         json.RawMessage `json:"alias,omitempty"`
	Explain       string          `json:"explain,omitempty"`
	Stem          string          `json:"stem"`
	StemMedia     json.RawMessage `json:"stem_media,omitempty"`
	Options       json.RawMessage `json:"options,omitempty"`
	DurationLimit *int            `json:"duration_limit,omitempty"`
}

// SessionQuestion 是下发给客户端的题目（不含答案与解析）。
type SessionQuestion struct {
	ItemID        string          `json:"item_id"`
	QuestionID    string          `json:"question_id"`
	Position      int             `json:"position"`
	Type          string          `json:"type"`
	Stem          string          `json:"stem"`
	StemMedia     json.RawMessage `json:"stem_media,omitempty"`
	Options       json.RawMessage `json:"options,omitempty"`
	DurationLimit *int            `json:"duration_limit,omitempty"`
	Token         string          `json:"token"`
	Answered      bool            `json:"answered"`
}

// CreateSessionResponse 是创建会话的返回。
type CreateSessionResponse struct {
	SessionID     string            `json:"session_id"`
	ExpiresAt     time.Time         `json:"expires_at"`
	PassThreshold float64           `json:"pass_threshold"`
	RuleVersion   int               `json:"rule_version"`
	Questions     []SessionQuestion `json:"questions"`
}

// AnswerResult 是单题判分结果。
type AnswerResult struct {
	Correct       bool            `json:"correct"`
	CorrectAnswer json.RawMessage `json:"correct_answer"`
	Explain       string          `json:"explain,omitempty"`
	DurationMS    int             `json:"duration_ms"`
	Suspicious    bool            `json:"suspicious"`
	Score         float64         `json:"score"`
	Threshold     float64         `json:"threshold"`
	Answered      int             `json:"answered"`
	Total         int             `json:"total"`
	Remaining     int             `json:"remaining"`
}

// SessionSummary 是会话状态。
type SessionSummary struct {
	SessionID     string            `json:"session_id"`
	State         string            `json:"state"`
	Score         float64           `json:"score"`
	Threshold     float64           `json:"pass_threshold"`
	CorrectCount  int               `json:"correct_count"`
	AnsweredCount int               `json:"answered_count"`
	QuestionCount int               `json:"question_count"`
	StartedAt     time.Time         `json:"started_at"`
	ExpiresAt     time.Time         `json:"expires_at"`
	FinishedAt    *time.Time        `json:"finished_at,omitempty"`
	Questions     []SessionQuestion `json:"questions,omitempty"`
	Grant         *watch.Grant      `json:"grant,omitempty"`
	GrantError    string            `json:"grant_error,omitempty"`
}

// AnswerSubmission 是提交答案的入参。
type AnswerSubmission struct {
	Token  string          `json:"token"`
	Answer json.RawMessage `json:"answer"`
}
