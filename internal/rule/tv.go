package rule

import (
	"context"
	"encoding/json"
	"time"

	"github.com/roma007/OpenQuizGo/internal/identity"
)

// TVContextResult 是电视端当前观看判定（081：按电视配置）。
type TVContextResult struct {
	Allowed          bool       `json:"allowed"`
	Reason           string     `json:"reason"`
	HardStop         bool       `json:"hard_stop"`
	QuizEnabled      bool       `json:"quiz_enabled"`
	LimitEnabled     bool       `json:"limit_enabled"`
	QuizScope        string     `json:"quiz_scope"`
	UnlockMinutes    int        `json:"unlock_minutes"`
	HardStopGrace    int        `json:"hard_stop_grace"`
	PassThreshold    float64    `json:"pass_threshold"`
	QuestionCount    int        `json:"question_count"`
	PassQuestions    int        `json:"pass_questions"`
	Subjects         []string   `json:"subjects"`
	DailyLimit       int        `json:"daily_limit"`
	UsedMinutes      int        `json:"used_minutes"`
	ReservedMinutes  int        `json:"reserved_minutes"`
	RemainingMinutes int        `json:"remaining_minutes"`
	ConfigVersion    int        `json:"config_version"`
	ChildID          string     `json:"child_id,omitempty"`
	ChildName        string     `json:"child_name,omitempty"`
	GrantMinutes     int        `json:"grant_minutes"`
	GrantUntil       *time.Time `json:"grant_until,omitempty"`
	LastQuizAt       *time.Time `json:"last_quiz_at,omitempty"`
	NextQuizAt       *time.Time `json:"next_quiz_at,omitempty"`
	Now              time.Time  `json:"now"`
}

// GetTVConfig 返回家长端某台电视的配置。
func (s *Service) GetTVConfig(ctx context.Context, accountID, devicePK string) (identity.TVConfig, error) {
	cfg, err := s.repo.tvConfigForParent(ctx, accountID, devicePK)
	if isNoRows(err) {
		return identity.TVConfig{}, newError(404, "TV_CONFIG_NOT_FOUND", "电视配置不存在")
	}
	return cfg, err
}

// UpdateTVConfig 更新某台电视的配置（需家长 PIN，version 乐观锁）。
func (s *Service) UpdateTVConfig(ctx context.Context, accountID, devicePK, pin string, in identity.TVConfig) (identity.TVConfig, error) {
	if err := s.verifyPIN(ctx, accountID, pin); err != nil {
		return identity.TVConfig{}, err
	}
	before, err := s.repo.tvConfigForParent(ctx, accountID, devicePK)
	if isNoRows(err) {
		return identity.TVConfig{}, newError(404, "TV_CONFIG_NOT_FOUND", "电视配置不存在")
	}
	if err != nil {
		return identity.TVConfig{}, err
	}
	// 旧版家长端不发送 question_count（0），沿用库中原值而不是重置为默认 1。
	if in.QuestionCount == 0 {
		in.QuestionCount = before.QuestionCount
	}
	if in.PassQuestions == 0 {
		in.PassQuestions = before.PassQuestions
	}
	in = normalizeTVConfig(in)
	if err := validateTVConfig(in); err != nil {
		return identity.TVConfig{}, err
	}
	// 客户端带上版本号时做乐观锁校验，避免两个家长端互相覆盖。
	if in.Version != 0 && in.Version != before.Version {
		return identity.TVConfig{}, newError(409, "TV_CONFIG_CONFLICT", "配置已被修改，请刷新后重试")
	}
	version, err := s.repo.updateTVConfig(ctx, accountID, devicePK, in, before.Version)
	if err == errVersionConflict {
		return identity.TVConfig{}, newError(409, "TV_CONFIG_CONFLICT", "配置已被修改，请刷新后重试")
	}
	if err != nil {
		return identity.TVConfig{}, err
	}
	in.Version = version
	return in, nil
}

// ContextForDevice 按电视配置给出当前可看状态与硬停判定。
func (s *Service) ContextForDevice(ctx context.Context, principal identity.DevicePrincipal) (TVContextResult, error) {
	cfg, err := s.repo.tvConfigByDevicePK(ctx, principal.DevicePK)
	if err != nil {
		return TVContextResult{}, newError(404, "TV_CONFIG_NOT_FOUND", "电视配置不存在")
	}
	base, _, _ := s.repo.getForChild(ctx, principal.ChildID)

	now := time.Now()
	start, end, err := DayBounds(base, now)
	if err != nil {
		return TVContextResult{}, err
	}
	used, reserved, err := s.repo.usageSecondsByDevice(ctx, principal.DeviceID, start, end)
	if err != nil {
		return TVContextResult{}, err
	}

	allowedByTime := true
	if len(base.TimeWindow.Allow) > 0 {
		allowedByTime, err = AllowedAt(base, now)
		if err != nil {
			return TVContextResult{}, err
		}
	}

	remaining := cfg.DailyLimit - ceilMinutes(used+reserved)
	if !cfg.LimitEnabled {
		remaining = cfg.DailyLimit
	}
	if remaining < 0 {
		remaining = 0
	}

	result := TVContextResult{
		Reason:           "ok",
		QuizEnabled:      cfg.QuizEnabled,
		LimitEnabled:     cfg.LimitEnabled,
		QuizScope:        cfg.QuizScope,
		UnlockMinutes:    cfg.UnlockMinutes,
		HardStopGrace:    cfg.HardStopGrace,
		PassThreshold:    effectivePassThreshold(applyTVConfig(base, cfg), cfg.QuestionCount),
		QuestionCount:    cfg.QuestionCount,
		PassQuestions:    min(cfg.PassQuestions, cfg.QuestionCount),
		Subjects:         cfg.Subjects,
		DailyLimit:       cfg.DailyLimit,
		UsedMinutes:      ceilMinutes(used),
		ReservedMinutes:  ceilMinutes(reserved),
		RemainingMinutes: remaining,
		ConfigVersion:    cfg.Version,
		ChildID:          principal.ChildID,
		ChildName:        principal.ChildName,
		Now:              now,
	}
	result.HardStop = cfg.LimitEnabled && remaining <= 0
	result.Allowed = allowedByTime && !result.HardStop
	switch {
	case result.HardStop:
		result.Reason = "daily_limit"
	case !allowedByTime:
		result.Reason = "time_window"
	}

	// 家长放行期间不再硬停：放行时段由家长 PIN 显式授予，即使当日额度已用完也照常生效。
	graceMinutes, grantUntil, err := s.repo.activeTVGrant(ctx, principal.DeviceID)
	if err != nil {
		return TVContextResult{}, err
	}
	if grantUntil != nil {
		result.HardStop = false
		result.Allowed = allowedByTime
		result.GrantMinutes = graceMinutes
		result.GrantUntil = grantUntil
		switch {
		case !allowedByTime:
			result.Reason = "time_window"
		default:
			result.Reason = "grant"
		}
	}

	// 下次强制答题时间 = 最近一次通过 + 强制答题间隔；状态页据此显示倒计时。
	lastPassed, err := s.repo.lastPassedQuizAt(ctx, principal.DeviceID, start)
	if err != nil {
		return TVContextResult{}, err
	}
	if lastPassed != nil {
		next := lastPassed.Add(time.Duration(cfg.UnlockMinutes) * time.Minute)
		result.LastQuizAt = lastPassed
		result.NextQuizAt = &next
	}
	return result, nil
}

// TVConfigForDeviceID 返回某台电视的配置（供计量服务判定硬停放行时长）。
func (s *Service) TVConfigForDeviceID(ctx context.Context, deviceID string) (identity.TVConfig, bool, error) {
	cfg, err := s.repo.tvConfigByDeviceID(ctx, deviceID)
	if isNoRows(err) {
		return identity.TVConfig{}, false, nil
	}
	if err != nil {
		return identity.TVConfig{}, false, err
	}
	return cfg, true, nil
}

// TVStartPlan 是电视发起答题前的配置快照（081）。
type TVStartPlan struct {
	Rule          Rule
	Version       int
	Random        bool
	Subjects      []string
	QuestionCount int
	PassQuestions int
}

// CanStartDevice 在电视发起答题前做配置校验，并合成一份用于组卷的规则。
func (s *Service) CanStartDevice(ctx context.Context, principal identity.DevicePrincipal) (TVStartPlan, error) {
	cfg, err := s.repo.tvConfigByDevicePK(ctx, principal.DevicePK)
	if err != nil {
		return TVStartPlan{}, newError(404, "TV_CONFIG_NOT_FOUND", "电视配置不存在")
	}
	if !cfg.QuizEnabled {
		return TVStartPlan{}, newError(409, "QUIZ_DISABLED", "家长已关闭答题开关")
	}
	base, _, err := s.repo.getForChild(ctx, principal.ChildID)
	if err != nil {
		base = Normalize(Rule{})
	}

	now := time.Now()
	if len(base.TimeWindow.Allow) > 0 {
		ok, err := AllowedAt(base, now)
		if err != nil {
			return TVStartPlan{}, err
		}
		if !ok {
			return TVStartPlan{}, newError(403, "OUT_OF_TIME_WINDOW", "当前不在允许观看的时段")
		}
	}
	if cfg.LimitEnabled {
		start, end, err := DayBounds(base, now)
		if err != nil {
			return TVStartPlan{}, err
		}
		used, reserved, err := s.repo.usageSecondsByDevice(ctx, principal.DeviceID, start, end)
		if err != nil {
			return TVStartPlan{}, err
		}
		if cfg.DailyLimit-ceilMinutes(used+reserved) <= 0 {
			return TVStartPlan{}, newError(403, "DAILY_LIMIT_REACHED", "今天的观看额度已经用完")
		}
	}

	plan := applyTVConfig(base, cfg)
	plan.PassThreshold = effectivePassThreshold(plan, cfg.QuestionCount)
	return TVStartPlan{
		Rule:          plan,
		Version:       cfg.Version,
		Random:        cfg.QuizScope == "random",
		Subjects:      cfg.Subjects,
		QuestionCount: cfg.QuestionCount,
		PassQuestions: min(cfg.PassQuestions, cfg.QuestionCount),
	}, nil
}

// effectivePassThreshold 在题量较小时把不可达的门槛下调到该题量可拿到的最高分，
// 避免孩子无论怎么答都无法解锁（082）。
func effectivePassThreshold(r Rule, questionCount int) float64 {
	if questionCount <= 0 {
		return r.PassThreshold
	}
	maxScore := MaxScore(r, questionCount)
	if maxScore > 0 && r.PassThreshold > maxScore {
		return maxScore
	}
	return r.PassThreshold
}

// applyTVConfig 用电视配置覆盖孩子规则中的可配项，保留时段与学科权重。
func applyTVConfig(base Rule, cfg identity.TVConfig) Rule {
	out := base
	out.UnlockMinutes = cfg.UnlockMinutes
	out.PassThreshold = cfg.PassThreshold
	if cfg.LimitEnabled {
		out.DailyLimit = cfg.DailyLimit
	} else {
		out.DailyLimit = 720
	}
	if len(cfg.Subjects) > 0 {
		filtered := map[string]float64{}
		for _, subject := range cfg.Subjects {
			if weight, ok := base.SubjectWeight[subject]; ok && weight > 0 {
				filtered[subject] = weight
			}
		}
		if subject := base.SubjectWeight["fun"]; subject > 0 && out.FunEnabled {
			filtered["fun"] = subject
		}
		if len(filtered) > 0 {
			out.SubjectWeight = filtered
		}
	}
	if out.Timezone == "" {
		out.Timezone = "Asia/Shanghai"
	}
	return out
}

func normalizeTVConfig(in identity.TVConfig) identity.TVConfig {
	if in.UnlockMinutes == 0 {
		in.UnlockMinutes = 15
	}
	if in.DailyLimit == 0 {
		in.DailyLimit = 60
	}
	if in.HardStopGrace == 0 {
		in.HardStopGrace = 10
	}
	if in.QuestionCount == 0 {
		in.QuestionCount = 1
	}
	if in.PassQuestions == 0 {
		in.PassQuestions = min(2, in.QuestionCount)
	}
	if in.PassThreshold == 0 {
		in.PassThreshold = 3
	}
	if in.QuizScope == "" {
		in.QuizScope = "random"
	}
	if in.Subjects == nil {
		in.Subjects = []string{}
	}
	return in
}

func validateTVConfig(in identity.TVConfig) error {
	if in.UnlockMinutes < 1 || in.UnlockMinutes > 240 {
		return newError(422, "VALIDATION_ERROR", "强制答题间隔需在 1–240 分钟")
	}
	if in.DailyLimit < 1 || in.DailyLimit > 720 {
		return newError(422, "VALIDATION_ERROR", "每日上限需在 1–720 分钟")
	}
	if in.HardStopGrace < 1 || in.HardStopGrace > 120 {
		return newError(422, "VALIDATION_ERROR", "放行时长需在 1–120 分钟")
	}
	if in.PassThreshold <= 0 {
		return newError(422, "VALIDATION_ERROR", "通过门槛需大于 0")
	}
	if in.QuizScope != "random" && in.QuizScope != "grade" {
		return newError(422, "VALIDATION_ERROR", "出题模式不合法")
	}
	if in.QuestionCount < 1 || in.QuestionCount > 5 {
		return newError(422, "VALIDATION_ERROR", "每次答题题量需在 1–5 题")
	}
	if in.PassQuestions < 1 || in.PassQuestions > in.QuestionCount {
		return newError(422, "VALIDATION_ERROR", "答对题数需在 1 到每次题量之间")
	}
	return nil
}

/* ------------------------------ 仓储 ------------------------------ */

const tvConfigColumns = `
	quiz_enabled, limit_enabled, unlock_minutes, daily_limit, subjects::text,
	pass_threshold::float8, quiz_scope, hard_stop_grace, question_count,
	pass_questions, version`

type tvConfigScanner interface {
	Scan(dest ...any) error
}

func scanTVConfig(row tvConfigScanner) (identity.TVConfig, error) {
	var cfg identity.TVConfig
	var subjects string
	err := row.Scan(&cfg.QuizEnabled, &cfg.LimitEnabled, &cfg.UnlockMinutes, &cfg.DailyLimit,
		&subjects, &cfg.PassThreshold, &cfg.QuizScope, &cfg.HardStopGrace,
		&cfg.QuestionCount, &cfg.PassQuestions, &cfg.Version)
	if err != nil {
		return identity.TVConfig{}, err
	}
	cfg.Subjects = []string{}
	_ = json.Unmarshal([]byte(subjects), &cfg.Subjects)
	return cfg, nil
}

func (r *repository) tvConfigForParent(ctx context.Context, accountID, devicePK string) (identity.TVConfig, error) {
	return scanTVConfig(r.pool.QueryRow(ctx, `
		select `+tvConfigColumns+`
		from tv_config cfg
		join tv_device t on t.id = cfg.device_pk
		join family_member fm on fm.family_id = t.family_id
		where cfg.device_pk = $1 and fm.account_id = $2 and t.status = 'active'`,
		devicePK, accountID))
}

func (r *repository) tvConfigByDevicePK(ctx context.Context, devicePK string) (identity.TVConfig, error) {
	return scanTVConfig(r.pool.QueryRow(ctx,
		`select `+tvConfigColumns+` from tv_config where device_pk = $1`, devicePK))
}

func (r *repository) tvConfigByDeviceID(ctx context.Context, deviceID string) (identity.TVConfig, error) {
	return scanTVConfig(r.pool.QueryRow(ctx, `
		select `+tvConfigColumns+`
		from tv_config cfg
		join tv_device t on t.id = cfg.device_pk
		where t.device_id = $1 and t.status = 'active'`, deviceID))
}

// lastPassedQuizAt 返回设备当天最近一次答题通过的时间。
func (r *repository) lastPassedQuizAt(ctx context.Context, deviceID string, since time.Time) (*time.Time, error) {
	var at *time.Time
	err := r.pool.QueryRow(ctx, `
		select max(finished_at)
		from quiz_session
		where device_id = $1 and state = 'passed' and finished_at >= $2`,
		deviceID, since).Scan(&at)
	if err != nil {
		return nil, err
	}
	return at, nil
}

// activeTVGrant 返回设备上仍在有效期内的家长放行时长（分钟）。
func (r *repository) activeTVGrant(ctx context.Context, deviceID string) (int, *time.Time, error) {
	var seconds int
	var until *time.Time
	err := r.pool.QueryRow(ctx, `
		select greatest(minutes * 60 - consumed_seconds, 0)::int, valid_until
		from watch_grant
		where device_id = $1 and status = 'active' and valid_until > now()
		order by valid_until desc
		limit 1`, deviceID).Scan(&seconds, &until)
	if isNoRows(err) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, err
	}
	minutes := (seconds + 59) / 60
	return minutes, until, nil
}

var errVersionConflict = &Error{Status: 409, Code: "TV_CONFIG_CONFLICT", Message: "配置已被修改"}

func (r *repository) updateTVConfig(ctx context.Context, accountID, devicePK string, in identity.TVConfig, expectedVersion int) (int, error) {
	subjects, _ := json.Marshal(in.Subjects)
	var version int
	err := r.pool.QueryRow(ctx, `
		update tv_config cfg
		set quiz_enabled = $3,
		    limit_enabled = $4,
		    unlock_minutes = $5,
		    daily_limit = $6,
		    subjects = $7::jsonb,
		    pass_threshold = $8,
		    quiz_scope = $9,
		    hard_stop_grace = $10,
		    question_count = $11,
		    pass_questions = $12,
		    version = version + 1,
		    updated_by = $2,
		    updated_at = now()
		where cfg.device_pk = $1
		  and cfg.version = $13
		  and exists (
		      select 1 from tv_device t
		      join family_member fm on fm.family_id = t.family_id
		      where t.id = cfg.device_pk and fm.account_id = $2 and t.status = 'active'
		  )
		returning version`,
		devicePK, accountID, in.QuizEnabled, in.LimitEnabled, in.UnlockMinutes, in.DailyLimit,
		string(subjects), in.PassThreshold, in.QuizScope, in.HardStopGrace,
		in.QuestionCount, in.PassQuestions, expectedVersion).Scan(&version)
	if err != nil {
		if isNoRows(err) {
			return 0, errVersionConflict
		}
		return 0, err
	}
	return version, nil
}

func (r *repository) usageSecondsByDevice(ctx context.Context, deviceID string, start, end time.Time) (used, reserved int, err error) {
	if err = r.pool.QueryRow(ctx, `
		select coalesce(sum(watched_seconds), 0)::int
		from watch_log
		where device_id = $1 and start_at >= $2 and start_at < $3`,
		deviceID, start, end).Scan(&used); err != nil {
		return 0, 0, err
	}
	if err = r.pool.QueryRow(ctx, `
		select coalesce(sum(minutes * 60 - consumed_seconds), 0)::int
		from watch_grant
		where device_id = $1 and status = 'active' and valid_until > now()`,
		deviceID).Scan(&reserved); err != nil {
		return 0, 0, err
	}
	return used, reserved, nil
}
