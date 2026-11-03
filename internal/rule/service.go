package rule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/identity"
)

// Error 是规则业务错误。
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// PreviewResult 是规则 dry-run 结果。
type PreviewResult struct {
	N        int            `json:"n"`
	Quota    map[string]int `json:"quota"`
	FunSlots int            `json:"fun_slots"`
	MaxScore float64        `json:"max_score"`
	Warnings []string       `json:"warnings,omitempty"`
}

// ContextResult 是电视端可看状态。
type ContextResult struct {
	Allowed          bool      `json:"allowed"`
	Reason           string    `json:"reason"`
	Rule             Rule      `json:"rule"`
	RuleVersion      int       `json:"rule_version"`
	DailyLimit       int       `json:"daily_limit"`
	UsedMinutes      int       `json:"used_minutes"`
	ReservedMinutes  int       `json:"reserved_minutes"`
	RemainingMinutes int       `json:"remaining_minutes"`
	Now              time.Time `json:"now"`
}

// Service 提供规则读取、更新、dry-run 与判定。
type Service struct {
	repo     *repository
	identity *identity.Service
	logger   *slog.Logger
}

// New 构造规则服务。
func New(pool *pgxpool.Pool, identityService *identity.Service, logger *slog.Logger) *Service {
	return &Service{repo: newRepository(pool), identity: identityService, logger: logger}
}

// Get 返回孩子规则。
func (s *Service) Get(ctx context.Context, accountID, childID string) (Rule, int, error) {
	r, version, err := s.repo.getForParent(ctx, accountID, childID)
	if isNoRows(err) {
		return Rule{}, 0, newError(404, "RULE_NOT_FOUND", "规则不存在")
	}
	return r, version, err
}

// Update 更新规则（需家长 PIN）。
func (s *Service) Update(ctx context.Context, accountID, childID, pin string, in Rule, ip string) (Rule, int, error) {
	in = Normalize(in)
	if err := s.validate(ctx, in); err != nil {
		return Rule{}, 0, err
	}
	if err := s.verifyPIN(ctx, accountID, pin); err != nil {
		return Rule{}, 0, err
	}
	before, _, err := s.repo.getForParent(ctx, accountID, childID)
	if isNoRows(err) {
		return Rule{}, 0, newError(404, "RULE_NOT_FOUND", "规则不存在")
	}
	if err != nil {
		return Rule{}, 0, err
	}
	version, err := s.repo.update(ctx, accountID, childID, in)
	if isNoRows(err) {
		return Rule{}, 0, newError(404, "RULE_NOT_FOUND", "规则不存在")
	}
	if err != nil {
		return Rule{}, 0, err
	}
	if err := s.repo.audit(ctx, accountID, childID, before, in, ip); err != nil {
		s.logger.Error("audit rule update", "error", err, "child_id", childID)
	}
	return in, version, nil
}

// Preview 校验规则并给出会话题量预估。
func (s *Service) Preview(ctx context.Context, accountID, childID string, in Rule) (PreviewResult, error) {
	in = Normalize(in)
	if _, _, err := s.repo.getForParent(ctx, accountID, childID); err != nil {
		if isNoRows(err) {
			return PreviewResult{}, newError(404, "RULE_NOT_FOUND", "规则不存在")
		}
		return PreviewResult{}, err
	}
	if err := s.validate(ctx, in); err != nil {
		return PreviewResult{}, err
	}
	n, _ := SessionSize(in)
	funSlots := 0
	if in.FunEnabled && in.FunRatioCap > 0 && in.SubjectWeight["fun"] > 0 {
		funSlots = min(in.FunRatioCap, max(1, n-1))
	}
	weights := make(map[string]float64)
	for subject, weight := range in.SubjectWeight {
		if subject != "fun" && weight > 0 {
			weights[subject] = weight
		}
	}
	quota := Allocate(weights, n-funSlots)
	maxScore := float64(funSlots) * in.SubjectWeight["fun"]
	for subject, count := range quota {
		maxScore += float64(count) * in.SubjectWeight[subject]
	}
	result := PreviewResult{N: n, Quota: quota, FunSlots: funSlots, MaxScore: maxScore}
	if in.PassThreshold > maxScore*0.9 {
		result.Warnings = append(result.Warnings, "门槛接近可达上限，孩子可能需要全部答对")
	}
	return result, nil
}

// Context 返回电视端当前可看状态。
func (s *Service) Context(ctx context.Context, childID string) (ContextResult, error) {
	r, version, err := s.repo.getForChild(ctx, childID)
	if isNoRows(err) {
		return ContextResult{}, newError(404, "RULE_NOT_FOUND", "规则不存在")
	}
	if err != nil {
		return ContextResult{}, err
	}
	now := time.Now()
	allowedByTime, err := AllowedAt(r, now)
	if err != nil {
		return ContextResult{}, err
	}
	start, end, err := DayBounds(r, now)
	if err != nil {
		return ContextResult{}, err
	}
	used, reserved, err := s.repo.usageSeconds(ctx, childID, start, end)
	if err != nil {
		return ContextResult{}, err
	}
	remaining := r.DailyLimit - ceilMinutes(used+reserved)

	result := ContextResult{
		Allowed:          allowedByTime && remaining > 0,
		Reason:           "ok",
		Rule:             r,
		RuleVersion:      version,
		DailyLimit:       r.DailyLimit,
		UsedMinutes:      ceilMinutes(used),
		ReservedMinutes:  ceilMinutes(reserved),
		RemainingMinutes: max(0, remaining),
		Now:              now,
	}
	if !allowedByTime {
		result.Reason = "time_window"
	} else if remaining <= 0 {
		result.Reason = "daily_limit"
	}
	return result, nil
}

// CanStart 在创建答题会话前执行时段与额度校验。
func (s *Service) CanStart(ctx context.Context, childID string) (Rule, int, error) {
	r, version, err := s.repo.getForChild(ctx, childID)
	if isNoRows(err) {
		return Rule{}, 0, newError(404, "RULE_NOT_FOUND", "规则不存在")
	}
	if err != nil {
		return Rule{}, 0, err
	}
	now := time.Now()
	allowed, err := AllowedAt(r, now)
	if err != nil {
		return Rule{}, 0, err
	}
	if !allowed {
		return Rule{}, 0, newError(403, "RULE_TIME_WINDOW", "当前不在允许观看时段")
	}
	start, end, err := DayBounds(r, now)
	if err != nil {
		return Rule{}, 0, err
	}
	used, reserved, err := s.repo.usageSeconds(ctx, childID, start, end)
	if err != nil {
		return Rule{}, 0, err
	}
	if r.DailyLimit-ceilMinutes(used+reserved) <= 0 {
		return Rule{}, 0, newError(403, "RULE_DAILY_LIMIT", "今日观看额度已用完")
	}
	return r, version, nil
}

func (s *Service) validate(ctx context.Context, in Rule) error {
	if err := Validate(in); err != nil {
		return newError(422, "VALIDATION_ERROR", err.Error())
	}
	subjects, err := s.repo.validSubjects(ctx)
	if err != nil {
		return err
	}
	for subject := range in.SubjectWeight {
		if !subjects[subject] {
			return newError(422, "VALIDATION_ERROR", fmt.Sprintf("学科不存在或已禁用：%s", subject))
		}
	}
	return nil
}

func (s *Service) verifyPIN(ctx context.Context, accountID, pin string) error {
	if s.identity == nil {
		return nil
	}
	if err := s.identity.VerifyPIN(ctx, accountID, pin); err != nil {
		var identityErr *identity.Error
		if errors.As(err, &identityErr) {
			if identityErr.Code == "PIN_NOT_SET" {
				return newError(403, "PIN_REQUIRED", "请先设置家长 PIN")
			}
			if identityErr.Code == "PIN_INVALID" {
				return newError(403, "PIN_INVALID", "家长 PIN 不正确")
			}
		}
		return err
	}
	return nil
}

func ceilMinutes(seconds int) int {
	if seconds <= 0 {
		return 0
	}
	return (seconds + 59) / 60
}
