package watch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const grantColumns = `
	id, session_id, child_id, device_id, minutes, valid_from, valid_until,
	jti, consumed_seconds, status, last_heartbeat_at, created_at`

type grantRecord struct {
	ID              string
	SessionID       string
	ChildID         string
	DeviceID        string
	Minutes         int
	ValidFrom       time.Time
	ValidUntil      time.Time
	JTI             string
	ConsumedSeconds int
	Status          string
	LastHeartbeatAt time.Time
	CreatedAt       time.Time
}

func scanGrant(row pgx.Row) (grantRecord, error) {
	var g grantRecord
	err := row.Scan(&g.ID, &g.SessionID, &g.ChildID, &g.DeviceID, &g.Minutes,
		&g.ValidFrom, &g.ValidUntil, &g.JTI, &g.ConsumedSeconds, &g.Status,
		&g.LastHeartbeatAt, &g.CreatedAt)
	return g, err
}

type repository struct {
	pool *pgxpool.Pool
}

func newRepository(pool *pgxpool.Pool) *repository {
	return &repository{pool: pool}
}

// issue 在事务内串行化同一孩子的授权签发，避免并发超发。
func (r *repository) issue(ctx context.Context, req IssueRequest, start, end time.Time,
	jti string, now time.Time) (grantRecord, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return grantRecord{}, false, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext($1))`, req.ChildID); err != nil {
		return grantRecord{}, false, fmt.Errorf("lock child: %w", err)
	}

	existing, err := scanGrant(tx.QueryRow(ctx,
		`select `+grantColumns+` from watch_grant where session_id = $1`, req.SessionID))
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return grantRecord{}, false, err
		}
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return grantRecord{}, false, err
	}

	var used, reserved int
	if err := tx.QueryRow(ctx, `
		select coalesce(sum(watched_seconds), 0)::int
		from watch_log where child_id = $1 and start_at >= $2 and start_at < $3`,
		req.ChildID, start, end).Scan(&used); err != nil {
		return grantRecord{}, false, err
	}
	if err := tx.QueryRow(ctx, `
		select coalesce(sum(minutes * 60 - consumed_seconds), 0)::int
		from watch_grant
		where child_id = $1 and status = 'active' and valid_until > $2`,
		req.ChildID, now).Scan(&reserved); err != nil {
		return grantRecord{}, false, err
	}

	dailySeconds := req.Rule.DailyLimit * 60
	if req.Rule.DailyLimit <= 0 {
		dailySeconds = 60 * 60
	}
	available := dailySeconds - used - reserved
	minutes := req.Rule.UnlockMinutes
	if minutes <= 0 {
		minutes = 20
	}
	if availMinutes := available / 60; availMinutes < minutes {
		minutes = availMinutes
	}
	if minutes < 1 {
		return grantRecord{}, false, newError(403, "RULE_DAILY_LIMIT", "今日观看额度已用完")
	}

	validUntil := now.Add(time.Duration(minutes) * time.Minute)
	rec, err := scanGrant(tx.QueryRow(ctx, `
		insert into watch_grant (
			child_id, device_id, session_id, minutes, valid_from, valid_until, jti,
			consumed_seconds, status, last_heartbeat_at)
		values ($1, $2, $3, $4, $5, $6, $7, 0, 'active', $5)
		returning `+grantColumns,
		req.ChildID, req.DeviceID, req.SessionID, minutes, now, validUntil, jti))
	if err != nil {
		return grantRecord{}, false, fmt.Errorf("insert grant: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return grantRecord{}, false, err
	}
	return rec, true, nil
}

func (r *repository) grantBySession(ctx context.Context, sessionID string) (grantRecord, error) {
	return scanGrant(r.pool.QueryRow(ctx,
		`select `+grantColumns+` from watch_grant where session_id = $1`, sessionID))
}

func (r *repository) activeGrant(ctx context.Context, deviceID, childID string, now time.Time) (grantRecord, error) {
	return scanGrant(r.pool.QueryRow(ctx, `
		select `+grantColumns+` from watch_grant
		where device_id = $1 and child_id = $2 and status = 'active' and valid_until > $3
		order by created_at desc limit 1`, deviceID, childID, now))
}

type heartbeatOutcome struct {
	Grant      grantRecord
	WatchedSec int
	PausedSec  int
	Revoked    bool
	Reason     string
}

func (r *repository) heartbeat(ctx context.Context, grantID, deviceID, childID string,
	allowedByTime bool, clientPaused bool, now time.Time) (heartbeatOutcome, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return heartbeatOutcome{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rec, err := scanGrant(tx.QueryRow(ctx, `
		select `+grantColumns+` from watch_grant
		where id = $1 and device_id = $2 and child_id = $3
		for update`, grantID, deviceID, childID))
	if err != nil {
		return heartbeatOutcome{}, err
	}

	outcome := heartbeatOutcome{Grant: rec}
	if rec.Status != "active" {
		return outcome, tx.Commit(ctx)
	}
	if now.After(rec.ValidUntil) {
		if _, err := tx.Exec(ctx, `update watch_grant set status = 'expired' where id = $1`, rec.ID); err != nil {
			return outcome, err
		}
		rec.Status = "expired"
		outcome.Grant = rec
		outcome.Reason = "expired"
		return outcome, tx.Commit(ctx)
	}
	if !allowedByTime {
		if _, err := tx.Exec(ctx, `update watch_grant set status = 'revoked' where id = $1`, rec.ID); err != nil {
			return outcome, err
		}
		rec.Status = "revoked"
		outcome.Grant = rec
		outcome.Revoked = true
		outcome.Reason = "time_window"
		return outcome, tx.Commit(ctx)
	}

	quizActive := false
	if err := tx.QueryRow(ctx, `
		select exists (
			select 1 from quiz_session
			where device_id = $1 and child_id = $2 and state = 'active'
		)`, deviceID, childID).Scan(&quizActive); err != nil {
		return outcome, err
	}

	delta := int(now.Sub(rec.LastHeartbeatAt).Seconds())
	if delta < 0 {
		delta = 0
	}
	if delta > 120 {
		delta = 120
	}
	paused := clientPaused || quizActive
	watched := 0
	pausedSec := 0
	if paused {
		pausedSec = delta
	} else {
		watched = delta
	}
	remaining := rec.Minutes*60 - rec.ConsumedSeconds
	if watched > remaining {
		watched = remaining
	}
	consumed := rec.ConsumedSeconds + watched
	status := rec.Status
	if consumed >= rec.Minutes*60 {
		status = "exhausted"
	}

	if _, err := tx.Exec(ctx, `
		update watch_grant
		set consumed_seconds = $2, status = $3, last_heartbeat_at = $4
		where id = $1`, rec.ID, consumed, status, now); err != nil {
		return outcome, err
	}

	if delta > 0 {
		if _, err := tx.Exec(ctx, `
			insert into watch_log (grant_id, child_id, device_id, target_type, target_id,
			                       start_at, end_at, watched_seconds, paused_seconds)
			values ($1, $2, $3, 'app', $4, $5, $6, $7, $8)`,
			rec.ID, childID, deviceID, "", rec.LastHeartbeatAt, now, watched, pausedSec); err != nil {
			return outcome, err
		}
	}

	rec.ConsumedSeconds = consumed
	rec.Status = status
	rec.LastHeartbeatAt = now
	outcome.Grant = rec
	outcome.WatchedSec = watched
	outcome.PausedSec = pausedSec
	return outcome, tx.Commit(ctx)
}

func (r *repository) upsertCapability(ctx context.Context, deviceID string, c Capability) error {
	permissions, _ := json.Marshal(c.Permissions)
	_, err := r.pool.Exec(ctx, `
		insert into device_capability (device_id, guard_mode, permissions, os_version, app_version, last_reported_at)
		values ($1, $2, $3::jsonb, $4, $5, now())
		on conflict (device_id) do update set
			guard_mode = excluded.guard_mode,
			permissions = excluded.permissions,
			os_version = excluded.os_version,
			app_version = excluded.app_version,
			last_reported_at = now()`,
		deviceID, c.GuardMode, string(permissions), c.OSVersion, c.AppVersion)
	return err
}

func (r *repository) insertTamper(ctx context.Context, deviceID, childID string, event TamperEvent) error {
	detail, _ := json.Marshal(event.Detail)
	_, err := r.pool.Exec(ctx, `
		insert into tamper_event (device_id, child_id, type, detail)
		values ($1, $2, $3, $4::jsonb)`, deviceID, childID, event.Type, string(detail))
	return err
}

type leaseRecord struct {
	ID              string
	DeviceID        string
	ChildID         string
	JTI             string
	MaxMinutes      int
	AllowedApps     []string
	IssuedAt        time.Time
	ExpiresAt       time.Time
	ReportedMinutes int
	Status          string
}

const leaseColumns = `
	id, device_id, child_id, jti, max_minutes, allowed_apps::text,
	issued_at, expires_at, reported_minutes, status`

func scanLease(row pgx.Row) (leaseRecord, error) {
	var lease leaseRecord
	var apps string
	err := row.Scan(&lease.ID, &lease.DeviceID, &lease.ChildID, &lease.JTI,
		&lease.MaxMinutes, &apps, &lease.IssuedAt, &lease.ExpiresAt,
		&lease.ReportedMinutes, &lease.Status)
	if err != nil {
		return leaseRecord{}, err
	}
	_ = json.Unmarshal([]byte(apps), &lease.AllowedApps)
	return lease, nil
}

func (r *repository) issueLease(ctx context.Context, childID, deviceID string, maxMinutes int,
	apps []string, jti string, issued, expires time.Time) (leaseRecord, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return leaseRecord{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		update offline_lease set status = 'expired'
		where device_id = $1 and status = 'active'`, deviceID); err != nil {
		return leaseRecord{}, err
	}
	encodedApps, _ := json.Marshal(apps)
	lease, err := scanLease(tx.QueryRow(ctx, `
		insert into offline_lease (device_id, child_id, jti, max_minutes, allowed_apps, issued_at, expires_at)
		values ($1, $2, $3, $4, $5::jsonb, $6, $7)
		returning `+leaseColumns,
		deviceID, childID, jti, maxMinutes, string(encodedApps), issued, expires))
	if err != nil {
		return leaseRecord{}, fmt.Errorf("insert lease: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return leaseRecord{}, err
	}
	return lease, nil
}

func (r *repository) leaseByID(ctx context.Context, id string) (leaseRecord, error) {
	return scanLease(r.pool.QueryRow(ctx,
		`select `+leaseColumns+` from offline_lease where id = $1`, id))
}

func (r *repository) markLeaseReported(ctx context.Context, id string, minutes int) error {
	_, err := r.pool.Exec(ctx, `
		update offline_lease set reported_minutes = $2, status = 'reported' where id = $1`, id, minutes)
	return err
}

func (r *repository) insertOfflineLog(ctx context.Context, childID, deviceID, target string,
	start, end time.Time, watched, paused int) error {
	_, err := r.pool.Exec(ctx, `
		insert into watch_log (child_id, device_id, target_type, target_id,
		                       start_at, end_at, watched_seconds, paused_seconds)
		values ($1, $2, 'app', $3, $4, $5, $6, $7)`,
		childID, deviceID, target, start, end, watched, paused)
	return err
}
