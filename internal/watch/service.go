package watch

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/platform/metrics"
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
	repo     *repository
	ruleSvc  *rule.Service
	secret   []byte
	leaseKey ed25519.PrivateKey
	leasePub string
	logger   *slog.Logger
}

const devLeaseKey = "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXphYmNkZWY="

// New 构造计量服务；leaseKeyBase64 为 Ed25519 32 字节种子。
func New(pool *pgxpool.Pool, ruleSvc *rule.Service, secret, leaseKeyBase64 string, logger *slog.Logger) (*Service, error) {
	if leaseKeyBase64 == "" {
		leaseKeyBase64 = devLeaseKey
	}
	seed, err := base64.StdEncoding.DecodeString(leaseKeyBase64)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("OFFLINE_LEASE_KEY must be base64 of 32 bytes")
	}
	key := ed25519.NewKeyFromSeed(seed)
	publicKey := key.Public().(ed25519.PublicKey)
	return &Service{
		repo:     newRepository(pool),
		ruleSvc:  ruleSvc,
		secret:   []byte(secret),
		leaseKey: key,
		leasePub: base64.StdEncoding.EncodeToString(publicKey),
		logger:   logger,
	}, nil
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
	rec, created, err := s.repo.issue(ctx, req, start, end, jti, now)
	if err != nil {
		return Grant{}, err
	}
	if created {
		metrics.IncGrantIssued()
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
	metrics.AddWatchedSeconds(outcome.WatchedSec)
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

// IssueOfflineLease 预签发离线租约（24 小时有效，最大离线分钟数受规则限制）。
func (s *Service) IssueOfflineLease(ctx context.Context, principal identity.DevicePrincipal) (OfflineLease, error) {
	if s.ruleSvc == nil {
		return OfflineLease{}, newError(500, "INTERNAL", "规则服务不可用")
	}
	state, err := s.ruleSvc.Context(ctx, principal.ChildID)
	if err != nil {
		var ruleErr *rule.Error
		if errors.As(err, &ruleErr) {
			return OfflineLease{}, newError(ruleErr.Status, ruleErr.Code, ruleErr.Message)
		}
		return OfflineLease{}, err
	}
	if !state.Allowed {
		code := "RULE_TIME_WINDOW"
		message := "当前不在允许观看时段"
		if state.Reason == "daily_limit" {
			code = "RULE_DAILY_LIMIT"
			message = "今日观看额度已用完"
		}
		return OfflineLease{}, newError(403, code, message)
	}
	maxMinutes := state.Rule.OfflineFallbackMin
	if maxMinutes <= 0 {
		return OfflineLease{}, newError(403, "OFFLINE_DISABLED", "未开启离线观看")
	}
	if maxMinutes > state.RemainingMinutes {
		maxMinutes = state.RemainingMinutes
	}
	if maxMinutes < 1 {
		return OfflineLease{}, newError(403, "RULE_DAILY_LIMIT", "今日剩余额度不足")
	}
	allowedApps := []string{}
	if state.Rule.ScopeMode == "custom" {
		allowedApps = state.Rule.AllowedApps
	}

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	lease, err := s.repo.issueLease(ctx, principal.ChildID, principal.DeviceID,
		maxMinutes, allowedApps, randomToken(), now, expiresAt)
	if err != nil {
		return OfflineLease{}, err
	}
	token, err := s.signOfflineLease(lease)
	if err != nil {
		return OfflineLease{}, err
	}
	return OfflineLease{
		LeaseID:     lease.ID,
		Token:       token,
		PublicKey:   s.leasePub,
		MaxMinutes:  lease.MaxMinutes,
		AllowedApps: lease.AllowedApps,
		IssuedAt:    lease.IssuedAt,
		ExpiresAt:   lease.ExpiresAt,
	}, nil
}

// ReportOffline 对账离线观看记录，超出额度的部分会被截断。
func (s *Service) ReportOffline(ctx context.Context, principal identity.DevicePrincipal,
	req OfflineReportRequest) (OfflineReportResult, error) {
	if req.LeaseID == "" || len(req.Entries) == 0 {
		return OfflineReportResult{}, newError(422, "VALIDATION_ERROR", "缺少租约或观看记录")
	}
	lease, err := s.repo.leaseByID(ctx, req.LeaseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OfflineReportResult{}, newError(404, "LEASE_NOT_FOUND", "离线租约不存在")
		}
		return OfflineReportResult{}, err
	}
	if lease.DeviceID != principal.DeviceID || lease.ChildID != principal.ChildID {
		return OfflineReportResult{}, newError(404, "LEASE_NOT_FOUND", "离线租约不存在")
	}
	if lease.Status == "reported" {
		return OfflineReportResult{}, newError(409, "LEASE_ALREADY_REPORTED", "该租约已对账")
	}

	now := time.Now()
	capSeconds := lease.MaxMinutes * 60
	inserted := 0
	clamped := false
	for _, entry := range req.Entries {
		if entry.WatchedSeconds < 0 || entry.PausedSeconds < 0 || !entry.EndAt.After(entry.StartAt) {
			return OfflineReportResult{}, newError(422, "VALIDATION_ERROR", "观看记录时间或秒数不合法")
		}
		if entry.StartAt.Before(lease.IssuedAt.Add(-5*time.Minute)) ||
			entry.EndAt.After(lease.ExpiresAt.Add(5*time.Minute)) ||
			entry.EndAt.After(now.Add(5*time.Minute)) {
			return OfflineReportResult{}, newError(422, "VALIDATION_ERROR", "观看记录不在租约有效期内")
		}
		duration := int(entry.EndAt.Sub(entry.StartAt).Seconds())
		if entry.WatchedSeconds+entry.PausedSeconds > duration+60 {
			return OfflineReportResult{}, newError(422, "VALIDATION_ERROR", "观看时长超过记录区间")
		}
		watched := entry.WatchedSeconds
		if inserted+watched > capSeconds {
			watched = capSeconds - inserted
			clamped = true
		}
		if watched < 0 {
			watched = 0
		}
		if watched == 0 && entry.PausedSeconds == 0 {
			continue
		}
		if err := s.repo.insertOfflineLog(ctx, lease.ChildID, lease.DeviceID, entry.TargetApp,
			entry.StartAt, entry.EndAt, watched, entry.PausedSeconds); err != nil {
			return OfflineReportResult{}, err
		}
		inserted += watched
	}

	minutes := (inserted + 59) / 60
	if err := s.repo.markLeaseReported(ctx, lease.ID, minutes); err != nil {
		return OfflineReportResult{}, err
	}

	remaining := 0
	if s.ruleSvc != nil {
		if state, err := s.ruleSvc.Context(ctx, lease.ChildID); err == nil {
			remaining = state.RemainingMinutes
		}
	}
	return OfflineReportResult{
		ReportedMinutes:  minutes,
		RemainingMinutes: remaining,
		Clamped:          clamped,
	}, nil
}

type offlineClaims struct {
	Type        string   `json:"typ"`
	ChildID     string   `json:"child_id"`
	DeviceID    string   `json:"device_id"`
	MaxMinutes  int      `json:"max_minutes"`
	AllowedApps []string `json:"allowed_apps,omitempty"`
	jwt.RegisteredClaims
}

func (s *Service) signOfflineLease(lease leaseRecord) (string, error) {
	claims := offlineClaims{
		Type:        "offline_lease",
		ChildID:     lease.ChildID,
		DeviceID:    lease.DeviceID,
		MaxMinutes:  lease.MaxMinutes,
		AllowedApps: lease.AllowedApps,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   lease.ID,
			ID:        lease.JTI,
			IssuedAt:  jwt.NewNumericDate(lease.IssuedAt),
			ExpiresAt: jwt.NewNumericDate(lease.ExpiresAt),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(s.leaseKey)
}
