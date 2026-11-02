package identity

import "time"

// Account 是家长账号的对外视图。
type Account struct {
	ID       string `json:"id"`
	FamilyID string `json:"family_id"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
}

// Child 是孩子档案的对外视图。
type Child struct {
	ID         string    `json:"id"`
	Nickname   string    `json:"nickname"`
	BirthYear  *int16    `json:"birth_year,omitempty"`
	BirthMonth *int16    `json:"birth_month,omitempty"`
	Stage      string    `json:"stage"`
	GradeTag   string    `json:"grade_tag,omitempty"`
	AvatarURL  string    `json:"avatar_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Device 是已绑定设备的对外视图。
type Device struct {
	ID        string     `json:"id"`
	DeviceID  string     `json:"device_id"`
	Platform  string     `json:"platform"`
	ChildID   string     `json:"child_id"`
	ChildName string     `json:"child_name"`
	BoundAt   time.Time  `json:"bound_at"`
	UnboundAt *time.Time `json:"unbound_at,omitempty"`
}

// Member 是家庭成员视图（手机号脱敏）。
type Member struct {
	AccountID string    `json:"account_id"`
	Phone     string    `json:"phone"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

// LoginResult 是登录/刷新令牌的返回。
type LoginResult struct {
	Account      Account   `json:"account"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// ActivationCode 是电视端申请到的激活码。
type ActivationCode struct {
	Code      string `json:"code"`
	ExpiresIn int64  `json:"expires_in"`
}

// DeviceToken 是设备令牌兑换结果。
type DeviceToken struct {
	DeviceID  string    `json:"device_id"`
	ChildID   string    `json:"child_id"`
	ChildName string    `json:"child_name"`
	Token     string    `json:"device_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Invite 是家庭邀请码。
type Invite struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DevicePrincipal 是设备鉴权后的上下文主体。
type DevicePrincipal struct {
	BindID    string
	DeviceID  string
	ChildID   string
	ChildName string
}

// Error 是带错误码与 HTTP 状态的业务错误。
type Error struct {
	Code    string
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(code string, status int, message string) *Error {
	return &Error{Code: code, Status: status, Message: message}
}

var (
	ErrInvalidPhone      = newError("VALIDATION_ERROR", 422, "手机号格式不正确")
	ErrInvalidCode       = newError("AUTH_INVALID_CODE", 401, "验证码无效或已过期")
	ErrRateLimited       = newError("RATE_LIMITED", 429, "操作过于频繁，请稍后再试")
	ErrSMSDisabled       = newError("SMS_DISABLED", 503, "短信服务未启用")
	ErrInvalidToken      = newError("AUTH_INVALID_TOKEN", 401, "令牌无效或已过期")
	ErrForbidden         = newError("FORBIDDEN", 403, "无权访问该资源")
	ErrNotFound          = newError("NOT_FOUND", 404, "资源不存在")
	ErrPINNotSet         = newError("PIN_NOT_SET", 409, "尚未设置家长 PIN")
	ErrPINInvalid        = newError("PIN_INVALID", 403, "家长 PIN 不正确")
	ErrInviteInvalid     = newError("INVITE_INVALID", 409, "邀请码无效或已过期")
	ErrChildStageInvalid = newError("VALIDATION_ERROR", 422, "学段取值不合法")
)
