package report

import "time"

// DailyStat 是单日答题统计。
type DailyStat struct {
	Date     string `json:"date"`
	Answered int    `json:"answered"`
	Correct  int    `json:"correct"`
}

// SubjectStat 是单学科统计。
type SubjectStat struct {
	Answered int         `json:"answered"`
	Correct  int         `json:"correct"`
	Accuracy float64     `json:"accuracy"`
	Trend    []DailyStat `json:"trend,omitempty"`
}

// Report 是成长报告。
type Report struct {
	ID           string                 `json:"id"`
	ChildID      string                 `json:"child_id"`
	Period       string                 `json:"period"`
	PeriodStart  time.Time              `json:"period_start"`
	PeriodEnd    time.Time              `json:"period_end"`
	SubjectStats map[string]SubjectStat `json:"subject_stats"`
	FunStats     SubjectStat            `json:"fun_stats"`
	WatchMinutes int                    `json:"watch_minutes"`
	GeneratedAt  time.Time              `json:"generated_at"`
}

// Error 是报告业务错误。
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}
