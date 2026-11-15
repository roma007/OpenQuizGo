package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

/* ------------------------------ 类型 ------------------------------ */

// TVConfig 是单台电视的观看配置（081）。
type TVConfig struct {
	QuizEnabled   bool     `json:"quiz_enabled"`
	LimitEnabled  bool     `json:"limit_enabled"`
	UnlockMinutes int      `json:"unlock_minutes"`
	DailyLimit    int      `json:"daily_limit"`
	Subjects      []string `json:"subjects"`
	PassThreshold float64  `json:"pass_threshold"`
	QuizScope     string   `json:"quiz_scope"`
	HardStopGrace int      `json:"hard_stop_grace"`
	QuestionCount int      `json:"question_count"`
	PassQuestions int      `json:"pass_questions"`
	Version       int      `json:"version"`
}

// TVDevice 是家长端看到的电视条目。
type TVDevice struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	DeviceID    string     `json:"device_id"`
	ChildID     string     `json:"child_id,omitempty"`
	ChildName   string     `json:"child_name,omitempty"`
	GradeTag    string     `json:"grade_tag,omitempty"`
	Online      bool       `json:"online"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	UsedMinutes int        `json:"used_minutes"`
	Config      TVConfig   `json:"config"`
}

// TVLoginInput 是电视端登录入参。
type TVLoginInput struct {
	Phone      string `json:"phone"`
	Code       string `json:"code"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// TVLoginResult 是电视端登录结果。
type TVLoginResult struct {
	DeviceToken string    `json:"device_token"`
	ExpiresAt   time.Time `json:"expires_at"`
	Device      TVDevice  `json:"device"`
	FamilyName  string    `json:"family_name"`
}

/* ------------------------------ 服务 ------------------------------ */

// TVLogin 校验短信验证码，把电视登记到该手机号所属家庭并签发设备令牌。
func (s *Service) TVLogin(ctx context.Context, in TVLoginInput) (TVLoginResult, error) {
	if in.DeviceID == "" {
		return TVLoginResult{}, newError("VALIDATION_ERROR", 422, "缺少设备标识")
	}
	normalized, err := normalizePhone(in.Phone)
	if err != nil {
		return TVLoginResult{}, err
	}
	phoneHash := s.cipher.Hash(normalized)
	if err := s.verifyLoginCode(ctx, phoneHash, in.Code); err != nil {
		return TVLoginResult{}, err
	}

	rec, err := s.repo.accountByPhoneHash(ctx, phoneHash)
	if err != nil {
		rec, err = s.createAccount(ctx, normalized, phoneHash)
		if err != nil {
			return TVLoginResult{}, err
		}
	}

	name := in.DeviceName
	if name == "" {
		name = "电视"
	}
	token := randomString(32)
	expiresAt := time.Now().Add(deviceTokenTTL)
	devicePK, err := s.repo.upsertTVDevice(ctx, rec.FamilyID, in.DeviceID, name, hashToken(token), expiresAt)
	if err != nil {
		return TVLoginResult{}, err
	}
	if err := s.repo.ensureTVConfig(ctx, devicePK); err != nil {
		return TVLoginResult{}, err
	}
	if err := s.repo.ensureTVChild(ctx, devicePK, rec.ID, name); err != nil {
		return TVLoginResult{}, err
	}

	device, err := s.repo.tvDeviceByPK(ctx, devicePK, rec.FamilyID)
	if err != nil {
		return TVLoginResult{}, err
	}
	familyName, _ := s.repo.familyName(ctx, rec.FamilyID)
	return TVLoginResult{
		DeviceToken: token,
		ExpiresAt:   expiresAt,
		Device:      device,
		FamilyName:  familyName,
	}, nil
}

// ListTVs 返回家庭下所有电视。
func (s *Service) ListTVs(ctx context.Context, accountID string) ([]TVDevice, error) {
	familyID, err := s.repo.familyIDForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return s.repo.listTVDevices(ctx, familyID)
}

// TVByID 返回家庭内某台电视。
func (s *Service) TVByID(ctx context.Context, accountID, devicePK string) (TVDevice, error) {
	familyID, err := s.repo.familyIDForAccount(ctx, accountID)
	if err != nil {
		return TVDevice{}, err
	}
	device, err := s.repo.tvDeviceByPK(ctx, devicePK, familyID)
	if err != nil {
		return TVDevice{}, newError("TV_NOT_FOUND", 404, "电视不存在")
	}
	return device, nil
}

// AssignTVChild 给电视指定孩子（昵称 + 年级）；gradeTag 为空表示仅建档案不分年级。
func (s *Service) AssignTVChild(ctx context.Context, accountID, devicePK, nickname, gradeTag string) (TVDevice, error) {
	familyID, err := s.repo.familyIDForAccount(ctx, accountID)
	if err != nil {
		return TVDevice{}, err
	}
	if _, err := s.repo.tvDeviceByPK(ctx, devicePK, familyID); err != nil {
		return TVDevice{}, newError("TV_NOT_FOUND", 404, "电视不存在")
	}
	if gradeTag != "" && !gradePattern.MatchString(gradeTag) {
		return TVDevice{}, newError("VALIDATION_ERROR", 422, "年级不合法")
	}
	if nickname == "" {
		nickname = "孩子"
	}
	if err := s.repo.assignTVChild(ctx, devicePK, accountID, nickname, gradeTag); err != nil {
		return TVDevice{}, err
	}
	return s.repo.tvDeviceByPK(ctx, devicePK, familyID)
}

// DeleteTV 吊销电视令牌。
func (s *Service) DeleteTV(ctx context.Context, accountID, devicePK string) error {
	familyID, err := s.repo.familyIDForAccount(ctx, accountID)
	if err != nil {
		return err
	}
	return s.repo.revokeTVDevice(ctx, devicePK, familyID)
}

// TVMe 返回设备自身的状态。
func (s *Service) TVMe(ctx context.Context, principal DevicePrincipal) (TVDevice, error) {
	return s.repo.tvDeviceByPK(ctx, principal.DevicePK, principal.FamilyID)
}

/* ------------------------------ 仓储 ------------------------------ */

func (r *repository) upsertTVDevice(ctx context.Context, familyID, deviceID, name, tokenHash string, expiresAt time.Time) (string, error) {
	const query = `
		insert into tv_device (family_id, device_id, name, platform, token_hash, token_expires_at, last_seen_at, status)
		values ($1, $2, $3, 'tv', $4, $5, now(), 'active')
		on conflict (device_id) where status = 'active'
		do update set
		    family_id = excluded.family_id,
		    name = excluded.name,
		    token_hash = excluded.token_hash,
		    token_expires_at = excluded.token_expires_at,
		    last_seen_at = now()
		returning id`
	var id string
	err := r.pool.QueryRow(ctx, query, familyID, deviceID, name, tokenHash, expiresAt).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert tv device: %w", err)
	}
	return id, nil
}

func (r *repository) ensureTVConfig(ctx context.Context, devicePK string) error {
	_, err := r.pool.Exec(ctx,
		`insert into tv_config (device_pk) values ($1) on conflict (device_pk) do nothing`, devicePK)
	if err != nil {
		return fmt.Errorf("ensure tv config: %w", err)
	}
	return nil
}

// ensureTVChild 在没有孩子时创建占位档案，并补一份默认 watch_rule。
func (r *repository) ensureTVChild(ctx context.Context, devicePK, accountID, nickname string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var childID string
	err = tx.QueryRow(ctx, `
		select coalesce(child_id::text, '') from tv_device where id = $1 for update`, devicePK).Scan(&childID)
	if err != nil {
		return fmt.Errorf("load tv child: %w", err)
	}
	if childID == "" {
		var familyID string
		if err := tx.QueryRow(ctx, `select family_id from tv_device where id = $1`, devicePK).Scan(&familyID); err != nil {
			return fmt.Errorf("load tv family: %w", err)
		}
		err = tx.QueryRow(ctx, `
			insert into child_profile (family_id, nickname, stage_code, is_placeholder)
			values ($1, $2, 'low', true)
			returning id`, familyID, nickname).Scan(&childID)
		if err != nil {
			return fmt.Errorf("insert placeholder child: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`insert into watch_rule (child_id, subject_weight, updated_by) values ($1, $2::jsonb, $3)`,
			childID, balancedWeights, accountID); err != nil {
			return fmt.Errorf("insert placeholder rule: %w", err)
		}
		if _, err := tx.Exec(ctx, `update tv_device set child_id = $2 where id = $1`, devicePK, childID); err != nil {
			return fmt.Errorf("attach tv child: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *repository) assignTVChild(ctx context.Context, devicePK, accountID, nickname, gradeTag string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var childID string
	var placeholder bool
	err = tx.QueryRow(ctx, `
		select coalesce(child_id::text, ''), coalesce((select is_placeholder from child_profile where id = tv_device.child_id), false)
		from tv_device where id = $1 for update`, devicePK).Scan(&childID, &placeholder)
	if err != nil {
		return fmt.Errorf("load tv child: %w", err)
	}

	grade := nullIfEmpty(gradeTag)
	if childID == "" {
		var familyID string
		if err := tx.QueryRow(ctx, `select family_id from tv_device where id = $1`, devicePK).Scan(&familyID); err != nil {
			return fmt.Errorf("load tv family: %w", err)
		}
		err = tx.QueryRow(ctx, `
			insert into child_profile (family_id, nickname, stage_code, grade_tag, grade_set_at)
			values ($1, $2, $3, $4, now())
			returning id`, familyID, nickname, gradeStage(gradeTag), grade).Scan(&childID)
		if err != nil {
			return fmt.Errorf("insert child: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`insert into watch_rule (child_id, subject_weight, updated_by) values ($1, $2::jsonb, $3)`,
			childID, balancedWeights, accountID); err != nil {
			return fmt.Errorf("insert child rule: %w", err)
		}
		if _, err := tx.Exec(ctx, `update tv_device set child_id = $2 where id = $1`, devicePK, childID); err != nil {
			return fmt.Errorf("attach child: %w", err)
		}
	} else if placeholder {
		// 占位档案直接改成真档案，历史记录一并归到孩子名下。
		if _, err := tx.Exec(ctx, `
			update child_profile
			set nickname = $2, stage_code = $3, grade_tag = $4, grade_set_at = now(),
			    is_placeholder = false, updated_at = now()
			where id = $1`, childID, nickname, gradeStage(gradeTag), grade); err != nil {
			return fmt.Errorf("update placeholder: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, `
			update child_profile
			set nickname = $2, stage_code = $3, grade_tag = $4, grade_set_at = now(),
			    updated_at = now()
			where id = $1`, childID, nickname, gradeStage(gradeTag), grade); err != nil {
			return fmt.Errorf("update child: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *repository) revokeTVDevice(ctx context.Context, devicePK, familyID string) error {
	_, err := r.pool.Exec(ctx, `
		update tv_device set status = 'revoked', token_hash = ''
		where id = $1 and family_id = $2 and status = 'active'`, devicePK, familyID)
	return err
}

func (r *repository) tvDeviceByToken(ctx context.Context, tokenHash string) (DevicePrincipal, error) {
	const query = `
		select t.id, t.family_id, t.device_id, coalesce(t.child_id::text, ''), coalesce(c.nickname, '')
		from tv_device t
		left join child_profile c on c.id = t.child_id
		where t.token_hash = $1
		  and t.status = 'active'
		  and t.token_expires_at > now()`
	var p DevicePrincipal
	err := r.pool.QueryRow(ctx, query, tokenHash).
		Scan(&p.DevicePK, &p.FamilyID, &p.DeviceID, &p.ChildID, &p.ChildName)
	p.Kind = "tv"
	return p, err
}

func (r *repository) touchTVDevice(ctx context.Context, devicePK string) {
	_, _ = r.pool.Exec(ctx, `update tv_device set last_seen_at = now() where id = $1`, devicePK)
}

func (r *repository) familyName(ctx context.Context, familyID string) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, `select name from family where id = $1`, familyID).Scan(&name)
	return name, err
}

const tvColumns = `
	t.id, t.name, t.device_id, coalesce(t.child_id::text, ''), coalesce(c.nickname, ''),
	coalesce(c.grade_tag, ''), t.last_seen_at,
	cfg.quiz_enabled, cfg.limit_enabled, cfg.unlock_minutes, cfg.daily_limit,
	cfg.subjects, cfg.pass_threshold::float8, cfg.quiz_scope, cfg.hard_stop_grace,
	cfg.question_count, cfg.pass_questions, cfg.version`

func (r *repository) scanTVDevice(row interface {
	Scan(dest ...any) error
}) (TVDevice, error) {
	var d TVDevice
	var subjects []byte
	err := row.Scan(&d.ID, &d.Name, &d.DeviceID, &d.ChildID, &d.ChildName, &d.GradeTag,
		&d.LastSeenAt, &d.Config.QuizEnabled, &d.Config.LimitEnabled, &d.Config.UnlockMinutes,
		&d.Config.DailyLimit, &subjects, &d.Config.PassThreshold, &d.Config.QuizScope,
		&d.Config.HardStopGrace, &d.Config.QuestionCount, &d.Config.PassQuestions,
		&d.Config.Version)
	if err != nil {
		return TVDevice{}, err
	}
	_ = json.Unmarshal(subjects, &d.Config.Subjects)
	if d.Config.Subjects == nil {
		d.Config.Subjects = []string{}
	}
	if d.LastSeenAt != nil {
		d.Online = time.Since(*d.LastSeenAt) < 3*time.Minute
	}
	return d, nil
}

func (r *repository) tvDeviceByPK(ctx context.Context, devicePK, familyID string) (TVDevice, error) {
	const query = `
		select ` + tvColumns + `
		from tv_device t
		left join child_profile c on c.id = t.child_id
		join tv_config cfg on cfg.device_pk = t.id
		where t.id = $1 and t.family_id = $2 and t.status = 'active'`
	return r.scanTVDevice(r.pool.QueryRow(ctx, query, devicePK, familyID))
}

func (r *repository) listTVDevices(ctx context.Context, familyID string) ([]TVDevice, error) {
	const query = `
		select ` + tvColumns + `
		from tv_device t
		left join child_profile c on c.id = t.child_id
		join tv_config cfg on cfg.device_pk = t.id
		where t.family_id = $1 and t.status = 'active'
		order by t.created_at asc`
	rows, err := r.pool.Query(ctx, query, familyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]TVDevice, 0, 4)
	for rows.Next() {
		d, err := r.scanTVDevice(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].UsedMinutes = r.tvUsedMinutes(ctx, items[i].DeviceID)
	}
	return items, nil
}

func (r *repository) tvUsedMinutes(ctx context.Context, deviceID string) int {
	var seconds int
	_ = r.pool.QueryRow(ctx, `
		select coalesce(sum(watched_seconds), 0)::int
		from watch_log
		where device_id = $1 and start_at >= date_trunc('day', now())`, deviceID).Scan(&seconds)
	return (seconds + 59) / 60
}

func gradeStage(gradeTag string) string {
	switch gradeTag {
	case "":
		return "low"
	case "g1", "g2":
		return "low"
	case "g3", "g4":
		return "mid"
	default:
		return "high"
	}
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
