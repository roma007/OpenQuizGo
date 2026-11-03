package watch

import "time"

// Grant 是服务端签发的观看授权。
type Grant struct {
	GrantID          string    `json:"grant_id"`
	SessionID        string    `json:"session_id"`
	Minutes          int       `json:"minutes"`
	ValidFrom        time.Time `json:"valid_from"`
	ValidUntil       time.Time `json:"valid_until"`
	RemainingSeconds int       `json:"remaining_seconds"`
	Status           string    `json:"status"`
	LeaseToken       string    `json:"lease_token,omitempty"`
}

// IssueRequest 是签发授权的入参。
type IssueRequest struct {
	SessionID string
	ChildID   string
	DeviceID  string
	Rule      RuleView
}

// RuleView 是计量所需的规则字段（避免 watch 直接依赖 quiz）。
type RuleView struct {
	UnlockMinutes int
	DailyLimit    int
	Timezone      string
}

// HeartbeatRequest 是心跳入参。
type HeartbeatRequest struct {
	ForegroundApp string `json:"foreground_app"`
	Paused        bool   `json:"paused"`
	PositionSec   int    `json:"position_seconds"`
}

// HeartbeatResult 是心跳结果。
type HeartbeatResult struct {
	GrantID          string    `json:"grant_id"`
	Status           string    `json:"status"`
	ConsumedSeconds  int       `json:"consumed_seconds"`
	RemainingSeconds int       `json:"remaining_seconds"`
	Paused           bool      `json:"paused"`
	Revoked          bool      `json:"revoked"`
	Reason           string    `json:"reason,omitempty"`
	ServerTime       time.Time `json:"server_time"`
}

// Error 是计量业务错误。
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// Capability 是设备能力上报。
type Capability struct {
	GuardMode   string          `json:"guard_mode"`
	Permissions map[string]bool `json:"permissions"`
	OSVersion   string          `json:"os_version"`
	AppVersion  string          `json:"app_version"`
}

// TamperEvent 是篡改/权限异常上报。
type TamperEvent struct {
	Type   string         `json:"type"`
	Detail map[string]any `json:"detail"`
}
