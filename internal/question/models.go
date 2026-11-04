package question

import (
	"encoding/json"
	"time"
)

// Question 是题目的管理端完整视图。
type Question struct {
	ID            string          `json:"id"`
	Subject       string          `json:"subject"`
	Category      string          `json:"category"`
	Stage         string          `json:"stage"`
	GradeTag      string          `json:"grade_tag,omitempty"`
	Difficulty    int             `json:"difficulty"`
	Type          string          `json:"type"`
	Stem          string          `json:"stem"`
	StemMedia     json.RawMessage `json:"stem_media,omitempty"`
	Options       json.RawMessage `json:"options,omitempty"`
	Answer        json.RawMessage `json:"answer"`
	AnswerAlias   json.RawMessage `json:"answer_alias,omitempty"`
	Explain       string          `json:"explain,omitempty"`
	DurationLimit *int            `json:"duration_limit,omitempty"`
	TopicID       string          `json:"topic_id,omitempty"`
	Status        string          `json:"status"`
	Version       int             `json:"version"`
	SchemaVer     int             `json:"schema_ver"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Input 是创建/更新题目的入参。
type Input struct {
	Subject       string          `json:"subject"`
	Category      string          `json:"category"`
	Stage         string          `json:"stage"`
	GradeTag      string          `json:"grade_tag"`
	Difficulty    int             `json:"difficulty"`
	Type          string          `json:"type"`
	Stem          string          `json:"stem"`
	StemMedia     json.RawMessage `json:"stem_media"`
	Options       json.RawMessage `json:"options"`
	Answer        json.RawMessage `json:"answer"`
	AnswerAlias   json.RawMessage `json:"answer_alias"`
	Explain       string          `json:"explain"`
	DurationLimit *int            `json:"duration_limit"`
	TopicID       *string         `json:"topic_id"`
	SchemaVer     int             `json:"schema_ver"`
}

// CoverageCell 是题库覆盖率聚合单元。
type CoverageCell struct {
	Subject    string `json:"subject"`
	Stage      string `json:"stage"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Difficulty int    `json:"difficulty"`
	Count      int    `json:"count"`
}

// CoverageResult 是覆盖率看板数据。
type CoverageResult struct {
	Cells []CoverageCell `json:"cells"`
	Total int            `json:"total"`
}

// Filter 是四维检索条件。
type Filter struct {
	Subject    string
	Category   string
	Stage      string
	Type       string
	Status     string
	Keyword    string
	TopicID    string
	Difficulty int
	Limit      int
	Offset     int
}

// ListResult 是分页结果。
type ListResult struct {
	Items []Question `json:"items"`
	Total int        `json:"total"`
}

// ImportError 是导入时单行校验错误。
type ImportError struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ImportResult 是导入结果。
type ImportResult struct {
	Imported int           `json:"imported"`
	Errors   []ImportError `json:"errors,omitempty"`
}

// AuditEntry 是审计日志视图。
type AuditEntry struct {
	ID         int64           `json:"id"`
	ActorType  string          `json:"actor_type"`
	ActorID    string          `json:"actor_id"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type"`
	TargetID   string          `json:"target_id"`
	After      json.RawMessage `json:"after_data,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
