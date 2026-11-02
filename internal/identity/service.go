package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/roma007/OpenQuizGo/internal/platform/cryptox"
)

const (
	smsCodeTTL       = 5 * time.Minute
	activationTTL    = 5 * time.Minute
	inviteTTL        = 7 * 24 * time.Hour
	deviceTokenTTL   = 365 * 24 * time.Hour
	smsPerHourLimit  = 5
	activationPerTTL = 10 * time.Second
)

var (
	phonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)
	gradePattern = regexp.MustCompile(`^g[1-9]$`)
	pinPattern   = regexp.MustCompile(`^\d{4,6}$`)
	validStages  = map[string]bool{"preschool": true, "low": true, "mid": true, "high": true}
	validPlat    = map[string]bool{"tv": true, "phone": true, "pad": true, "miniapp": true}
)

// Options 是身份服务配置。
type Options struct {
	JWTSecret   string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
	SMSMode     string
	SMSMockCode string
	Environment string
}

// ChildInput 是创建/更新孩子档案的入参。
type ChildInput struct {
	Nickname   string `json:"nickname"`
	BirthYear  *int16 `json:"birth_year"`
	BirthMonth *int16 `json:"birth_month"`
	Stage      string `json:"stage"`
	GradeTag   string `json:"grade_tag"`
	AvatarURL  string `json:"avatar_url"`
}

// Service 承载身份、家庭、孩子、设备与 PIN 的业务逻辑。
type Service struct {
	repo   *repository
	redis  *redis.Client
	cipher *cryptox.Cipher
	tokens *tokenManager
	logger *slog.Logger
	opts   Options
}

// NewService 构造身份服务。
func NewService(
	pool *pgxpool.Pool,
	rdb *redis.Client,
	cipher *cryptox.Cipher,
	opts Options,
	logger *slog.Logger,
) *Service {
	return &Service{
		repo:   newRepository(pool),
		redis:  rdb,
		cipher: cipher,
		tokens: newTokenManager(opts.JWTSecret, opts.AccessTTL, opts.RefreshTTL),
		logger: logger,
		opts:   opts,
	}
}

/* ------------------------------ 登录 ------------------------------ */

// SendLoginCode 下发登录验证码；mock 模式下写日志并返回固定验证码。
func (s *Service) SendLoginCode(ctx context.Context, phone string) error {
	normalized, err := normalizePhone(phone)
	if err != nil {
		return err
	}
	if s.opts.SMSMode == "disabled" {
		return ErrSMSDisabled
	}

	phoneHash := s.cipher.Hash(normalized)
	if err := s.checkSMSRate(ctx, phoneHash); err != nil {
		return err
	}

	code := s.opts.SMSMockCode
	if code == "" {
		code, err = randomDigits(6)
		if err != nil {
			return err
		}
	}
	if err := s.redis.Set(ctx, smsKey(phoneHash), code, smsCodeTTL).Err(); err != nil {
		return fmt.Errorf("store sms code: %w", err)
	}

	if s.opts.Environment == "dev" {
		s.logger.Info("sms code issued (mock)", "phone", maskPhone(normalized), "code", code)
	}
	return nil
}

// Login 校验验证码并登录；手机号首次登录时自动创建账号与家庭。
func (s *Service) Login(ctx context.Context, phone, code, userAgent, ip string) (LoginResult, error) {
	normalized, err := normalizePhone(phone)
	if err != nil {
		return LoginResult{}, err
	}
	phoneHash := s.cipher.Hash(normalized)

	stored, err := s.redis.GetDel(ctx, smsKey(phoneHash)).Result()
	if err != nil || subtle.ConstantTimeCompare([]byte(stored), []byte(code)) != 1 {
		return LoginResult{}, ErrInvalidCode
	}

	rec, err := s.repo.accountByPhoneHash(ctx, phoneHash)
	if errors.Is(err, pgx.ErrNoRows) {
		rec, err = s.createAccount(ctx, normalized, phoneHash)
	}
	if err != nil {
		return LoginResult{}, err
	}
	return s.issueLogin(ctx, rec, userAgent, ip)
}

func (s *Service) createAccount(ctx context.Context, phone, phoneHash string) (accountRecord, error) {
	cipherText, err := s.cipher.Encrypt(phone)
	if err != nil {
		return accountRecord{}, fmt.Errorf("encrypt phone: %w", err)
	}
	rec, err := s.repo.createAccount(ctx, cipherText, phoneHash)
	if err == nil {
		return rec, nil
	}
	// 并发首登可能触发唯一键冲突，回读已有账号。
	if existing, lookupErr := s.repo.accountByPhoneHash(ctx, phoneHash); lookupErr == nil {
		return existing, nil
	}
	return accountRecord{}, err
}

// Refresh 轮换刷新令牌并签发新的访问令牌。
func (s *Service) Refresh(ctx context.Context, refreshToken, userAgent, ip string) (LoginResult, error) {
	if refreshToken == "" {
		return LoginResult{}, ErrInvalidToken
	}
	raw, newHash, err := newRefreshToken()
	if err != nil {
		return LoginResult{}, err
	}
	expiresAt := time.Now().Add(s.opts.RefreshTTL)
	accountID, err := s.repo.rotateSession(ctx, hashToken(refreshToken), newHash, userAgent, ip, expiresAt)
	if err != nil {
		return LoginResult{}, err
	}
	rec, err := s.repo.accountByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LoginResult{}, ErrInvalidToken
		}
		return LoginResult{}, err
	}
	access, accessExp, err := s.tokens.issueAccess(rec.ID)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		Account:      s.accountView(rec),
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int64(s.opts.AccessTTL.Seconds()),
		ExpiresAt:    accessExp,
	}, nil
}

// Logout 吊销刷新令牌。
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.repo.revokeSession(ctx, hashToken(refreshToken))
}

// Me 返回当前家长账号。
func (s *Service) Me(ctx context.Context, accountID string) (Account, error) {
	rec, err := s.repo.accountByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Account{}, ErrInvalidToken
		}
		return Account{}, err
	}
	return s.accountView(rec), nil
}

func (s *Service) issueLogin(ctx context.Context, rec accountRecord, userAgent, ip string) (LoginResult, error) {
	access, accessExp, err := s.tokens.issueAccess(rec.ID)
	if err != nil {
		return LoginResult{}, err
	}
	raw, refreshHash, err := newRefreshToken()
	if err != nil {
		return LoginResult{}, err
	}
	if err := s.repo.createSession(ctx, rec.ID, refreshHash, userAgent, ip,
		time.Now().Add(s.opts.RefreshTTL)); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		Account:      s.accountView(rec),
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int64(s.opts.AccessTTL.Seconds()),
		ExpiresAt:    accessExp,
	}, nil
}

func (s *Service) accountView(rec accountRecord) Account {
	phone, err := s.cipher.Decrypt(rec.PhoneCipher)
	if err != nil {
		phone = ""
	}
	return Account{
		ID:       rec.ID,
		FamilyID: rec.FamilyID,
		Phone:    maskPhone(phone),
		Role:     rec.Role,
	}
}

/* ------------------------------ 孩子 ------------------------------ */

// ListChildren 返回当前家庭的孩子。
func (s *Service) ListChildren(ctx context.Context, accountID string) ([]Child, error) {
	return s.repo.listChildren(ctx, accountID)
}

// CreateChild 创建孩子档案并生成默认规则。
func (s *Service) CreateChild(ctx context.Context, accountID string, in ChildInput) (Child, error) {
	if err := validateChildInput(in); err != nil {
		return Child{}, err
	}
	return s.repo.createChild(ctx, accountID, in)
}

// UpdateChild 更新孩子档案。
func (s *Service) UpdateChild(ctx context.Context, accountID, childID string, in ChildInput) (Child, error) {
	if err := validateChildInput(in); err != nil {
		return Child{}, err
	}
	child, err := s.repo.updateChild(ctx, accountID, childID, in)
	if errors.Is(err, pgx.ErrNoRows) {
		return Child{}, ErrNotFound
	}
	return child, err
}

// GetChild 返回单个孩子。
func (s *Service) GetChild(ctx context.Context, accountID, childID string) (Child, error) {
	child, err := s.repo.getChild(ctx, accountID, childID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Child{}, ErrNotFound
	}
	return child, err
}

func validateChildInput(in ChildInput) error {
	in.Nickname = strings.TrimSpace(in.Nickname)
	if n := utf8.RuneCountInString(in.Nickname); n < 1 || n > 20 {
		return newError("VALIDATION_ERROR", 422, "昵称长度需在 1–20 字之间")
	}
	if !validStages[in.Stage] {
		return ErrChildStageInvalid
	}
	if in.GradeTag != "" && !gradePattern.MatchString(in.GradeTag) {
		return newError("VALIDATION_ERROR", 422, "年级标签不合法")
	}
	if (in.BirthYear == nil) != (in.BirthMonth == nil) {
		return newError("VALIDATION_ERROR", 422, "出生年月需同时提供或同时留空")
	}
	if in.BirthYear != nil {
		if *in.BirthYear < 2000 || *in.BirthYear > 2100 ||
			*in.BirthMonth < 1 || *in.BirthMonth > 12 {
			return newError("VALIDATION_ERROR", 422, "出生年月不合法")
		}
	}
	if len(in.AvatarURL) > 512 {
		return newError("VALIDATION_ERROR", 422, "头像地址过长")
	}
	return nil
}

/* ------------------------------ PIN ------------------------------ */

// SetPIN 设置或修改家长 PIN。
func (s *Service) SetPIN(ctx context.Context, accountID, current, next string) error {
	if !pinPattern.MatchString(next) {
		return newError("VALIDATION_ERROR", 422, "PIN 需为 4–6 位数字")
	}
	existing, err := s.repo.pinHash(ctx, accountID)
	if err != nil {
		return err
	}
	if existing != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(existing), []byte(current)); err != nil {
			return ErrPINInvalid
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash pin: %w", err)
	}
	return s.repo.setPIN(ctx, accountID, string(hash))
}

// VerifyPIN 校验家长 PIN。
func (s *Service) VerifyPIN(ctx context.Context, accountID, pin string) error {
	existing, err := s.repo.pinHash(ctx, accountID)
	if err != nil {
		return err
	}
	if existing == "" {
		return ErrPINNotSet
	}
	if err := bcrypt.CompareHashAndPassword([]byte(existing), []byte(pin)); err != nil {
		return ErrPINInvalid
	}
	return nil
}

/* ------------------------------ 设备 ------------------------------ */

// ListDevices 返回家庭当前绑定的设备。
func (s *Service) ListDevices(ctx context.Context, accountID string) ([]Device, error) {
	return s.repo.listDevices(ctx, accountID)
}

// CreateActivationCode 电视端申请激活码。
func (s *Service) CreateActivationCode(ctx context.Context, deviceID, platform string) (ActivationCode, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" || len(deviceID) > 128 {
		return ActivationCode{}, newError("VALIDATION_ERROR", 422, "device_id 不合法")
	}
	if !validPlat[platform] {
		return ActivationCode{}, newError("VALIDATION_ERROR", 422, "platform 不合法")
	}
	if ok, err := s.redis.SetNX(ctx, activationDeviceLockKey(deviceID), "1", activationPerTTL).Result(); err != nil {
		return ActivationCode{}, err
	} else if !ok {
		return ActivationCode{}, ErrRateLimited
	}

	oldCode, _ := s.redis.Get(ctx, activationDeviceKey(deviceID)).Result()
	if oldCode != "" {
		_ = s.redis.Del(ctx, activationKey(oldCode)).Err()
	}

	record := activationRecord{DeviceID: deviceID, Platform: platform, Status: "pending"}
	payload, _ := json.Marshal(record)

	var code string
	for i := 0; i < 5; i++ {
		candidate, err := randomDigits(6)
		if err != nil {
			return ActivationCode{}, err
		}
		ok, err := s.redis.SetNX(ctx, activationKey(candidate), payload, activationTTL).Result()
		if err != nil {
			return ActivationCode{}, err
		}
		if ok {
			code = candidate
			break
		}
	}
	if code == "" {
		return ActivationCode{}, newError("INTERNAL", 500, "生成激活码失败")
	}
	if err := s.redis.Set(ctx, activationDeviceKey(deviceID), code, activationTTL).Err(); err != nil {
		return ActivationCode{}, err
	}
	return ActivationCode{Code: code, ExpiresIn: int64(activationTTL.Seconds())}, nil
}

// ConfirmActivation 家长输入激活码并选择孩子。
func (s *Service) ConfirmActivation(ctx context.Context, accountID, code, childID string) (string, error) {
	record, err := s.loadActivation(ctx, code)
	if err != nil {
		return "", err
	}
	if record.Status != "pending" {
		return "", ErrInvalidCode
	}
	bindID, err := s.repo.bindDevice(ctx, accountID, record.DeviceID, childID, record.Platform)
	if err != nil {
		return "", err
	}
	record.Status = "confirmed"
	record.ChildID = childID
	record.BindID = bindID
	payload, _ := json.Marshal(record)
	if err := s.redis.Set(ctx, activationKey(code), payload, activationTTL).Err(); err != nil {
		return "", err
	}
	return record.DeviceID, nil
}

// ExchangeDeviceToken 电视端凭激活码兑换长期设备令牌。
func (s *Service) ExchangeDeviceToken(ctx context.Context, code, deviceID string) (DeviceToken, bool, error) {
	record, err := s.loadActivation(ctx, code)
	if err != nil {
		return DeviceToken{}, false, err
	}
	if record.DeviceID != strings.TrimSpace(deviceID) {
		return DeviceToken{}, false, ErrInvalidCode
	}
	if record.Status == "pending" {
		return DeviceToken{}, true, nil
	}

	token := randomString(32)
	expiresAt := time.Now().Add(deviceTokenTTL)
	if err := s.repo.setDeviceToken(ctx, record.BindID, hashToken(token), expiresAt); err != nil {
		return DeviceToken{}, false, err
	}
	_ = s.redis.Del(ctx, activationKey(code)).Err()
	_ = s.redis.Del(ctx, activationDeviceKey(deviceID)).Err()

	principal, err := s.repo.deviceByToken(ctx, hashToken(token))
	if err != nil {
		return DeviceToken{}, false, err
	}
	return DeviceToken{
		DeviceID:  principal.DeviceID,
		ChildID:   principal.ChildID,
		ChildName: principal.ChildName,
		Token:     token,
		ExpiresAt: expiresAt,
	}, false, nil
}

func (s *Service) loadActivation(ctx context.Context, code string) (activationRecord, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return activationRecord{}, ErrInvalidCode
	}
	payload, err := s.redis.Get(ctx, activationKey(code)).Result()
	if err != nil {
		return activationRecord{}, ErrInvalidCode
	}
	var record activationRecord
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return activationRecord{}, ErrInvalidCode
	}
	return record, nil
}

// UnbindDevice 解绑设备，设备令牌立即失效。
func (s *Service) UnbindDevice(ctx context.Context, accountID, bindID string) error {
	ok, err := s.repo.unbindDevice(ctx, accountID, bindID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

/* ------------------------------ 家庭成员 ------------------------------ */

// InviteMember 生成家庭邀请码。
func (s *Service) InviteMember(ctx context.Context, accountID string) (Invite, error) {
	familyID, err := s.repo.familyIDForAccount(ctx, accountID)
	if err != nil {
		return Invite{}, err
	}
	code, err := randomDigits(6)
	if err != nil {
		return Invite{}, err
	}
	expiresAt := time.Now().Add(inviteTTL)
	if err := s.repo.createInvite(ctx, familyID, accountID, code, expiresAt); err != nil {
		return Invite{}, err
	}
	return Invite{Code: code, ExpiresAt: expiresAt}, nil
}

// JoinFamily 使用邀请码加入家庭。
func (s *Service) JoinFamily(ctx context.Context, accountID, code string) (string, error) {
	return s.repo.joinByCode(ctx, strings.TrimSpace(code), accountID)
}

// ListMembers 返回家庭成员（手机号脱敏）。
func (s *Service) ListMembers(ctx context.Context, accountID string) ([]Member, error) {
	familyID, err := s.repo.familyIDForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.listMembers(ctx, familyID)
	if err != nil {
		return nil, err
	}
	for i := range members {
		if phone, err := s.cipher.Decrypt(members[i].Phone); err == nil {
			members[i].Phone = maskPhone(phone)
		} else {
			members[i].Phone = "***"
		}
	}
	return members, nil
}

/* ------------------------------ 辅助 ------------------------------ */

func (s *Service) checkSMSRate(ctx context.Context, phoneHash string) error {
	ok, err := s.redis.SetNX(ctx, smsMinuteKey(phoneHash), "1", time.Minute).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrRateLimited
	}
	count, err := s.redis.Incr(ctx, smsHourKey(phoneHash)).Result()
	if err != nil {
		return err
	}
	if count == 1 {
		_ = s.redis.Expire(ctx, smsHourKey(phoneHash), time.Hour).Err()
	}
	if count > smsPerHourLimit {
		return ErrRateLimited
	}
	return nil
}

func normalizePhone(raw string) (string, error) {
	phone := strings.NewReplacer(" ", "", "-", "", "+86", "").Replace(strings.TrimSpace(raw))
	if !phonePattern.MatchString(phone) {
		return "", ErrInvalidPhone
	}
	return phone, nil
}

func maskPhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}

func randomDigits(n int) (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	value, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", n, value), nil
}

type activationRecord struct {
	DeviceID string `json:"device_id"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
	ChildID  string `json:"child_id,omitempty"`
	BindID   string `json:"bind_id,omitempty"`
}

func smsKey(phoneHash string) string       { return "sms:login:" + phoneHash }
func smsMinuteKey(phoneHash string) string { return "rl:sms:min:" + phoneHash }
func smsHourKey(phoneHash string) string   { return "rl:sms:hour:" + phoneHash }
func activationKey(code string) string     { return "device:code:" + code }
func activationDeviceKey(id string) string { return "device:code:device:" + id }
func activationDeviceLockKey(id string) string {
	return "rl:device:code:" + id
}
