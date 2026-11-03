package watch

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/rule"
)

var validTamperTypes = map[string]bool{
	"permission_revoked": true,
	"uninstall_attempt":  true,
	"package_tampered":   true,
	"clock_skew":         true,
	"other":              true,
}

// Service 负责授权签发、心跳计量与设备异常上报。
type Service struct {
	repo    *repository
	ruleSvc *rule.Service
	secret  []byte
	logger  *slog.Logger
}

// New 构造计量服务。
func New(pool *pgxpool.Pool, ruleSvc *rule.Service, secret string, logger *slog.Logger) *Service {
	return &Service{repo: newRepository(pool), ruleSvc: ruleSvc, secret: []byte(secret), logger: logger}
}

// Issue 为通过的答题会话签发授权（同一会话幂等）。
func (s *Service) Issue(ctx context.Context, req IssueRequest) (Grant, error) {
	if req.SessionID == "" || req.ChildID == "" || req.DeviceID == "" {
		return Grant{}, newError(422, "VALIDATION_ERROR", "缺少会话、孩子或设备")
	}
	now := time.Now()
	location := time.UTC
	if req.Rule.Timezone != "" {
		if loc, err := time.LoadLocation(req.Rule.Timezone); err == nil {
			location = loc
		}
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	end := start.Add(24 * time.Hour)

	jti := randomToken()
	rec, _, err := s.repo.issue(ctx, req, start, end, jti, now)
	if err != nil {
		return Grant{}, err
	}
	return s.toGrant(rec), nil
}

// GrantForSession 返回会话已签发的授权。
func (s *Service) GrantForSession(ctx context.Context, sessionID string) (Grant, error) {
	rec, err := s.repo.grantBySession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Grant{}, newError(404, "GRANT_NOT_FOUND", "授权不存在")
		}
		return Grant{}, err
	}
	return s.toGrant(rec), nil
}

// ActiveGrant 返回设备当前有效授权。
func (s *Service) ActiveGrant(ctx context.Context, principal identity.DevicePrincipal) (Grant, error) {
	rec, err := s.repo.activeGrant(ctx, principal.DeviceID, principal.ChildID, time.Now())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Grant{}, newError(404, "GRANT_NOT_FOUND", "当前没有有效授权")
		}
		return Grant{}, err
	}
	return s.toGrant(rec), nil
}

// Heartbeat 计费心跳：答题期间与客户端暂停时不计入观看时长。
func (s *Service) Heartbeat(ctx context.Context, principal identity.DevicePrincipal,
	grantID string, req HeartbeatRequest) (HeartbeatResult, error) {
	now := time.Now()
	allowed := true
	if s.ruleSvc != nil {
		r, _, err := s.ruleSvc.GetForChild(ctx, principal.ChildID)
		if err == nil {
			allowed, _ = rule.AllowedAt(r, now)
		} else {
			var ruleErr *rule.Error
			if !errors.As(err, &ruleErr) || ruleErr.Code != "RULE_NOT_FOUND" {
				return HeartbeatResult{}, err
			}
		}
	}

	outcome, err := s.repo.heartbeat(ctx, grantID, principal.DeviceID, principal.ChildID,
		allowed, req.Paused, now)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return HeartbeatResult{}, newError(404, "GRANT_NOT_FOUND", "授权不存在")
		}
		return HeartbeatResult{}, err
	}
	remaining := outcome.Grant.Minutes*60 - outcome.Grant.ConsumedSeconds
	if remaining < 0 {
		remaining = 0
	}
	result := HeartbeatResult{
		GrantID:          outcome.Grant.ID,
		Status:           outcome.Grant.Status,
		ConsumedSeconds:  outcome.Grant.ConsumedSeconds,
		RemainingSeconds: remaining,
		Paused:           req.Paused || outcome.PausedSec > 0,
		Revoked:          outcome.Revoked || outcome.Grant.Status == "revoked",
		Reason:           outcome.Reason,
		ServerTime:       now,
	}
	return result, nil
}

// ReportCapability 记录设备防护模式与权限状态。
func (s *Service) ReportCapability(ctx context.Context, principal identity.DevicePrincipal, c Capability) error {
	if c.GuardMode == "" {
		c.GuardMode = "standard"
	}
	if c.GuardMode != "standard" && c.GuardMode != "device_owner" {
		return newError(422, "VALIDATION_ERROR", "guard_mode 不合法")
	}
	return s.repo.upsertCapability(ctx, principal.DeviceID, c)
}

// ReportTamper 记录权限关闭、卸载尝试等异常。
func (s *Service) ReportTamper(ctx context.Context, principal identity.DevicePrincipal, event TamperEvent) error {
	if !validTamperTypes[event.Type] {
		return newError(422, "VALIDATION_ERROR", "异常类型不合法")
	}
	return s.repo.insertTamper(ctx, principal.DeviceID, principal.ChildID, event)
}

func (s *Service) toGrant(rec grantRecord) Grant {
	remaining := rec.Minutes*60 - rec.ConsumedSeconds
	if remaining < 0 {
		remaining = 0
	}
	token, err := s.signLease(rec)
	if err != nil {
		s.logger.Error("sign lease", "error", err, "grant_id", rec.ID)
	}
	return Grant{
		GrantID:          rec.ID,
		SessionID:        rec.SessionID,
		Minutes:          rec.Minutes,
		ValidFrom:        rec.ValidFrom,
		ValidUntil:       rec.ValidUntil,
		RemainingSeconds: remaining,
		Status:           rec.Status,
		LeaseToken:       token,
	}
}

type leaseClaims struct {
	Type      string `json:"typ"`
	ChildID   string `json:"child_id"`
	DeviceID  string `json:"device_id"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

func (s *Service) signLease(rec grantRecord) (string, error) {
	claims := leaseClaims{
		Type:      "grant",
		ChildID:   rec.ChildID,
		DeviceID:  rec.DeviceID,
		SessionID: rec.SessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   rec.ID,
			ID:        rec.JTI,
			IssuedAt:  jwt.NewNumericDate(rec.ValidFrom),
			ExpiresAt: jwt.NewNumericDate(rec.ValidUntil),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func randomToken() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
