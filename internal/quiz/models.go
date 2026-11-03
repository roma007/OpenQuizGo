package quiz

import "time"

// SelectRequest 是组卷请求。
type SelectRequest struct {
	ChildID      string `json:"child_id"`
	Stage        string `json:"stage"`
	GradeTag     string `json:"grade_tag"`
	Rule         Rule   `json:"rule"`
	Seed         int64  `json:"seed"`
	MaxQuestions int    `json:"max_questions"`
}

// Selected 是被选中的题目。
type Selected struct {
	QuestionID string `json:"question_id"`
	Subject    string `json:"subject"`
	Stage      string `json:"stage"`
	Type       string `json:"type"`
	Difficulty int    `json:"difficulty"`
	Repeated   bool   `json:"repeated"`
}

// Result 是组卷结果。
type Result struct {
	Total            int            `json:"total"`
	Quota            map[string]int `json:"quota"`
	Selected         []Selected     `json:"selected"`
	MaxScore         float64        `json:"max_score"`
	DifficultyTarget int            `json:"difficulty_target"`
	Warnings         []string       `json:"warnings,omitempty"`
	Seed             int64          `json:"seed"`
}

// Error 是组卷业务错误。
type Error struct {
	Status    int
	Code      string
	Message   string
	Warnings  []string
	SessionID string
}

func (e *Error) Error() string { return e.Message }

func newError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// PreviewResponse 便于管理端 dry-run 查看。
type PreviewResponse struct {
	Result
	GeneratedAt time.Time `json:"generated_at"`
}
