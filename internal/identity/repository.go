package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const balancedWeights = `{"math":1.5,"chinese":1.5,"english":1.5,"science":1,"logic":1,"safety":1,"fun":0.5}`

type accountRecord struct {
	ID          string
	PhoneCipher string
	PINHash     string
	FamilyID    string
	Role        string
}

type repository struct {
	pool *pgxpool.Pool
}

func newRepository(pool *pgxpool.Pool) *repository {
	return &repository{pool: pool}
}

func (r *repository) accountByPhoneHash(ctx context.Context, phoneHash string) (accountRecord, error) {
	const query = `
		select pa.id, pa.phone_cipher, coalesce(pa.pin_hash, ''),
		       fm.family_id, fm.role
		from parent_account pa
		join family_member fm on fm.account_id = pa.id
		where pa.phone_hash = $1
		  and pa.status = 'active'
		  and (pa.active_family_id is null or fm.family_id = pa.active_family_id)
		order by (pa.active_family_id is not null) desc, fm.joined_at asc
		limit 1`

	var rec accountRecord
	err := r.pool.QueryRow(ctx, query, phoneHash).Scan(
		&rec.ID, &rec.PhoneCipher, &rec.PINHash, &rec.FamilyID, &rec.Role)
	if err != nil {
		return accountRecord{}, err
	}
	return rec, nil
}

func (r *repository) accountByID(ctx context.Context, accountID string) (accountRecord, error) {
	const query = `
		select pa.id, pa.phone_cipher, coalesce(pa.pin_hash, ''), fm.family_id, fm.role
		from parent_account pa
		join family_member fm on fm.account_id = pa.id
		where pa.id = $1
		  and pa.status = 'active'
		  and (pa.active_family_id is null or fm.family_id = pa.active_family_id)
		order by (pa.active_family_id is not null) desc, fm.joined_at asc
		limit 1`

	var rec accountRecord
	err := r.pool.QueryRow(ctx, query, accountID).Scan(
		&rec.ID, &rec.PhoneCipher, &rec.PINHash, &rec.FamilyID, &rec.Role)
	return rec, err
}

func (r *repository) createAccount(ctx context.Context, phoneCipher, phoneHash string) (accountRecord, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return accountRecord{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var accountID, familyID string
	if err := tx.QueryRow(ctx,
		`insert into parent_account (phone_cipher, phone_hash) values ($1, $2) returning id`,
		phoneCipher, phoneHash).Scan(&accountID); err != nil {
		return accountRecord{}, fmt.Errorf("insert account: %w", err)
	}
	if err := tx.QueryRow(ctx,
		`insert into family (owner_id) values ($1) returning id`, accountID).Scan(&familyID); err != nil {
		return accountRecord{}, fmt.Errorf("insert family: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`insert into family_member (family_id, account_id, role, second_factor_verified)
		 values ($1, $2, 'owner', true)`, familyID, accountID); err != nil {
		return accountRecord{}, fmt.Errorf("insert family member: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`update parent_account set active_family_id = $1 where id = $2`, familyID, accountID); err != nil {
		return accountRecord{}, fmt.Errorf("set active family: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return accountRecord{}, fmt.Errorf("commit: %w", err)
	}

	return accountRecord{
		ID:          accountID,
		PhoneCipher: phoneCipher,
		FamilyID:    familyID,
		Role:        "owner",
	}, nil
}

func (r *repository) familyIDForAccount(ctx context.Context, accountID string) (string, error) {
	var familyID string
	err := r.pool.QueryRow(ctx, `
		select coalesce(pa.active_family_id, fm.family_id)
		from parent_account pa
		join family_member fm on fm.account_id = pa.id
		where pa.id = $1
		order by (pa.active_family_id is not null) desc, fm.joined_at asc
		limit 1`, accountID).Scan(&familyID)
	return familyID, err
}

func (r *repository) listChildren(ctx context.Context, accountID string) ([]Child, error) {
	const query = `
		select c.id, c.nickname, c.birth_year, c.birth_month, c.stage_code,
		       coalesce(c.grade_tag, ''), coalesce(c.avatar_url, ''), c.created_at
		from child_profile c
		join family_member fm on fm.family_id = c.family_id
		where fm.account_id = $1
		  and c.family_id = coalesce(
		      (select active_family_id from parent_account where id = $1),
		      fm.family_id)
		order by c.created_at`

	rows, err := r.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("query children: %w", err)
	}
	defer rows.Close()

	children := make([]Child, 0)
	for rows.Next() {
		var c Child
		var birthYear, birthMonth *int16
		if err := rows.Scan(&c.ID, &c.Nickname, &birthYear, &birthMonth,
			&c.Stage, &c.GradeTag, &c.AvatarURL, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan child: %w", err)
		}
		c.BirthYear, c.BirthMonth = birthYear, birthMonth
		children = append(children, c)
	}
	return children, rows.Err()
}

func (r *repository) createChild(ctx context.Context, accountID string, in ChildInput) (Child, error) {
	familyID, err := r.familyIDForAccount(ctx, accountID)
	if err != nil {
		return Child{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Child{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertChild = `
		insert into child_profile
			(family_id, nickname, birth_year, birth_month, stage_code, grade_tag, avatar_url)
		values ($1, $2, $3, $4, $5, nullif($6, ''), nullif($7, ''))
		returning id, created_at`
	var child Child
	if err := tx.QueryRow(ctx, insertChild, familyID, in.Nickname, in.BirthYear, in.BirthMonth,
		in.Stage, in.GradeTag, in.AvatarURL).Scan(&child.ID, &child.CreatedAt); err != nil {
		return Child{}, fmt.Errorf("insert child: %w", err)
	}
	child.Nickname = in.Nickname
	child.BirthYear = in.BirthYear
	child.BirthMonth = in.BirthMonth
	child.Stage = in.Stage
	child.GradeTag = in.GradeTag
	child.AvatarURL = in.AvatarURL

	if _, err := tx.Exec(ctx,
		`insert into watch_rule (child_id, subject_weight, updated_by) values ($1, $2::jsonb, $3)`,
		child.ID, balancedWeights, accountID); err != nil {
		return Child{}, fmt.Errorf("insert default rule: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Child{}, fmt.Errorf("commit: %w", err)
	}
	return child, nil
}

func (r *repository) getChild(ctx context.Context, accountID, childID string) (Child, error) {
	const query = `
		select c.id, c.nickname, c.birth_year, c.birth_month, c.stage_code,
		       coalesce(c.grade_tag, ''), coalesce(c.avatar_url, ''), c.created_at
		from child_profile c
		join family_member fm on fm.family_id = c.family_id
		where c.id = $1 and fm.account_id = $2`

	var c Child
	var birthYear, birthMonth *int16
	err := r.pool.QueryRow(ctx, query, childID, accountID).Scan(
		&c.ID, &c.Nickname, &birthYear, &birthMonth, &c.Stage, &c.GradeTag, &c.AvatarURL, &c.CreatedAt)
	if err != nil {
		return Child{}, err
	}
	c.BirthYear, c.BirthMonth = birthYear, birthMonth
	return c, nil
}

func (r *repository) updateChild(ctx context.Context, accountID, childID string, in ChildInput) (Child, error) {
	const query = `
		update child_profile c
		set nickname = $3,
		    birth_year = $4,
		    birth_month = $5,
		    stage_code = $6,
		    grade_tag = nullif($7, ''),
		    avatar_url = nullif($8, ''),
		    updated_at = now()
		where c.id = $1
		  and exists (
		      select 1 from family_member fm
		      where fm.family_id = c.family_id and fm.account_id = $2
		  )
		returning c.id, c.nickname, c.birth_year, c.birth_month, c.stage_code,
		          coalesce(c.grade_tag, ''), coalesce(c.avatar_url, ''), c.created_at`

	var c Child
	var birthYear, birthMonth *int16
	err := r.pool.QueryRow(ctx, query, childID, accountID, in.Nickname, in.BirthYear, in.BirthMonth,
		in.Stage, in.GradeTag, in.AvatarURL).Scan(
		&c.ID, &c.Nickname, &birthYear, &birthMonth, &c.Stage, &c.GradeTag, &c.AvatarURL, &c.CreatedAt)
	if err != nil {
		return Child{}, err
	}
	c.BirthYear, c.BirthMonth = birthYear, birthMonth
	return c, nil
}

func (r *repository) pinHash(ctx context.Context, accountID string) (string, error) {
	var hash string
	err := r.pool.QueryRow(ctx,
		`select coalesce(pin_hash, '') from parent_account where id = $1`, accountID).Scan(&hash)
	return hash, err
}

func (r *repository) setPIN(ctx context.Context, accountID, hash string) error {
	_, err := r.pool.Exec(ctx,
		`update parent_account set pin_hash = $2, pin_updated_at = now(), updated_at = now() where id = $1`,
		accountID, hash)
	return err
}

func (r *repository) listDevices(ctx context.Context, accountID string) ([]Device, error) {
	const query = `
		select d.id, d.device_id, d.platform, d.child_id, c.nickname, d.bound_at
		from device_bind d
		join child_profile c on c.id = d.child_id
		join family_member fm on fm.family_id = c.family_id
		where fm.account_id = $1 and d.status = 'active'
		  and c.family_id = coalesce(
		      (select active_family_id from parent_account where id = $1),
		      fm.family_id)
		order by d.bound_at desc`

	rows, err := r.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()

	devices := make([]Device, 0)
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.DeviceID, &d.Platform, &d.ChildID, &d.ChildName, &d.BoundAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (r *repository) deviceForAccount(ctx context.Context, accountID, bindID string) (Device, error) {
	const query = `
		select d.id, d.device_id, d.platform, d.child_id, c.nickname, d.bound_at
		from device_bind d
		join child_profile c on c.id = d.child_id
		join family_member fm on fm.family_id = c.family_id
		where fm.account_id = $1 and d.id = $2 and d.status = 'active'
		  and c.family_id = coalesce(
		      (select active_family_id from parent_account where id = $1),
		      fm.family_id)
		limit 1`
	var d Device
	err := r.pool.QueryRow(ctx, query, accountID, bindID).
		Scan(&d.ID, &d.DeviceID, &d.Platform, &d.ChildID, &d.ChildName, &d.BoundAt)
	if err != nil {
		return Device{}, err
	}
	return d, nil
}

func (r *repository) bindDevice(ctx context.Context, accountID, deviceID, childID, platform string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `
		select exists (
			select 1 from child_profile c
			join family_member fm on fm.family_id = c.family_id
			where c.id = $1 and fm.account_id = $2
		)`, childID, accountID).Scan(&exists); err != nil {
		return "", fmt.Errorf("verify child: %w", err)
	}
	if !exists {
		return "", ErrForbidden
	}

	if _, err := tx.Exec(ctx, `
		update device_bind
		set status = 'unbound', unbound_at = now(), token_hash = '', token_expires_at = null
		where device_id = $1 and status = 'active'`, deviceID); err != nil {
		return "", fmt.Errorf("unbind previous: %w", err)
	}

	var bindID string
	if err := tx.QueryRow(ctx, `
		insert into device_bind (device_id, child_id, platform, status)
		values ($1, $2, $3, 'active')
		returning id`, deviceID, childID, platform).Scan(&bindID); err != nil {
		return "", fmt.Errorf("insert device bind: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return bindID, nil
}

func (r *repository) setDeviceToken(ctx context.Context, bindID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		update device_bind
		set token_hash = $2, token_expires_at = $3
		where id = $1 and status = 'active'`, bindID, tokenHash, expiresAt)
	return err
}

func (r *repository) deviceByToken(ctx context.Context, tokenHash string) (DevicePrincipal, error) {
	const query = `
		select d.id, d.device_id, d.child_id, c.nickname
		from device_bind d
		join child_profile c on c.id = d.child_id
		where d.token_hash = $1
		  and d.status = 'active'
		  and d.token_expires_at > now()`

	var p DevicePrincipal
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(&p.BindID, &p.DeviceID, &p.ChildID, &p.ChildName)
	return p, err
}

// familyOwnerForChild 返回孩子所属家庭的 owner 账号。
func (r *repository) familyOwnerForChild(ctx context.Context, childID string) (string, error) {
	var accountID string
	err := r.pool.QueryRow(ctx, `
		select f.owner_id
		from child_profile c
		join family f on f.id = c.family_id
		where c.id = $1`, childID).Scan(&accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return accountID, nil
}

func (r *repository) unbindDevice(ctx context.Context, accountID, bindID string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		update device_bind d
		set status = 'unbound', unbound_at = now(), token_hash = '', token_expires_at = null
		where d.id = $1
		  and d.status = 'active'
		  and exists (
		      select 1 from child_profile c
		      join family_member fm on fm.family_id = c.family_id
		      where c.id = d.child_id and fm.account_id = $2
		  )`, bindID, accountID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// unbindDeviceSelf 由设备自身调用，按 device_id 解除 active 绑定。
func (r *repository) unbindDeviceSelf(ctx context.Context, deviceID string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		update device_bind
		set status = 'unbound', unbound_at = now(), token_hash = '', token_expires_at = null
		where device_id = $1 and status = 'active'`, deviceID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *repository) createSession(ctx context.Context, accountID, tokenHash, userAgent, ip string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into parent_session (account_id, token_hash, user_agent, ip, expires_at)
		values ($1, $2, $3, nullif($4, '')::inet, $5)`,
		accountID, tokenHash, userAgent, ip, expiresAt)
	return err
}

func (r *repository) rotateSession(ctx context.Context, oldHash, newHash, userAgent, ip string, expiresAt time.Time) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var accountID string
	err = tx.QueryRow(ctx, `
		update parent_session
		set revoked_at = now()
		where token_hash = $1 and revoked_at is null and expires_at > now()
		returning account_id`, oldHash).Scan(&accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrInvalidToken
		}
		return "", fmt.Errorf("revoke old session: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into parent_session (account_id, token_hash, user_agent, ip, expires_at)
		values ($1, $2, $3, nullif($4, '')::inet, $5)`,
		accountID, newHash, userAgent, ip, expiresAt); err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return accountID, nil
}

func (r *repository) revokeSession(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx,
		`update parent_session set revoked_at = now() where token_hash = $1 and revoked_at is null`, tokenHash)
	return err
}

func (r *repository) createInvite(ctx context.Context, familyID, creatorID, code string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into family_invite (family_id, code, created_by, expires_at)
		values ($1, $2, $3, $4)`, familyID, code, creatorID, expiresAt)
	return err
}

func (r *repository) joinByCode(ctx context.Context, code, accountID string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var inviteID, familyID string
	err = tx.QueryRow(ctx, `
		select id, family_id from family_invite
		where code = $1 and used_at is null and expires_at > now()
		for update`, code).Scan(&inviteID, &familyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrInviteInvalid
		}
		return "", fmt.Errorf("load invite: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		insert into family_member (family_id, account_id, role, second_factor_verified)
		values ($1, $2, 'member', false)
		on conflict (family_id, account_id) do nothing`, familyID, accountID); err != nil {
		return "", fmt.Errorf("insert member: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`update parent_account set active_family_id = $1 where id = $2`, familyID, accountID); err != nil {
		return "", fmt.Errorf("switch family: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`update family_invite set used_at = now(), used_by = $2 where id = $1`, inviteID, accountID); err != nil {
		return "", fmt.Errorf("mark invite used: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return familyID, nil
}

func (r *repository) listMembers(ctx context.Context, familyID string) ([]Member, error) {
	rows, err := r.pool.Query(ctx, `
		select fm.account_id, pa.phone_cipher, fm.role, fm.joined_at
		from family_member fm
		join parent_account pa on pa.id = fm.account_id
		where fm.family_id = $1
		order by fm.joined_at`, familyID)
	if err != nil {
		return nil, fmt.Errorf("query members: %w", err)
	}
	defer rows.Close()

	members := make([]Member, 0)
	for rows.Next() {
		var m Member
		var phoneCipher string
		if err := rows.Scan(&m.AccountID, &phoneCipher, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		m.Phone = phoneCipher
		members = append(members, m)
	}
	return members, rows.Err()
}
