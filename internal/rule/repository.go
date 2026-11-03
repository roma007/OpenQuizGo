package rule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ruleColumns = `
	unlock_minutes, pass_threshold, subject_weight::text, fun_ratio_cap, fun_enabled,
	weight_preset, time_window::text, daily_limit, offline_fallback_min, scope_mode,
	allowed_apps::text, timezone, version`

type repository struct {
	pool *pgxpool.Pool
}

func newRepository(pool *pgxpool.Pool) *repository {
	return &repository{pool: pool}
}

func scanRule(row pgx.Row) (Rule, int, error) {
	var r Rule
	var version int
	var weights, timeWindow, apps string
	if err := row.Scan(
		&r.UnlockMinutes, &r.PassThreshold, &weights, &r.FunRatioCap, &r.FunEnabled,
		&r.WeightPreset, &timeWindow, &r.DailyLimit, &r.OfflineFallbackMin, &r.ScopeMode,
		&apps, &r.Timezone, &version); err != nil {
		return Rule{}, 0, err
	}
	r.SubjectWeight = map[string]float64{}
	if err := json.Unmarshal([]byte(weights), &r.SubjectWeight); err != nil {
		return Rule{}, 0, fmt.Errorf("decode subject_weight: %w", err)
	}
	if err := json.Unmarshal([]byte(timeWindow), &r.TimeWindow); err != nil {
		return Rule{}, 0, fmt.Errorf("decode time_window: %w", err)
	}
	if err := json.Unmarshal([]byte(apps), &r.AllowedApps); err != nil {
		return Rule{}, 0, fmt.Errorf("decode allowed_apps: %w", err)
	}
	return r, version, nil
}

func (r *repository) getForChild(ctx context.Context, childID string) (Rule, int, error) {
	return scanRule(r.pool.QueryRow(ctx,
		`select `+ruleColumns+` from watch_rule where child_id = $1`, childID))
}

func (r *repository) getForParent(ctx context.Context, accountID, childID string) (Rule, int, error) {
	return scanRule(r.pool.QueryRow(ctx, `
		select `+ruleColumns+`
		from watch_rule wr
		where wr.child_id = $1
		  and exists (
		      select 1 from child_profile c
		      join family_member fm on fm.family_id = c.family_id
		      where c.id = wr.child_id and fm.account_id = $2
		  )`, childID, accountID))
}

func (r *repository) update(ctx context.Context, accountID, childID string, in Rule) (int, error) {
	weights, _ := json.Marshal(in.SubjectWeight)
	timeWindow, _ := json.Marshal(in.TimeWindow)
	apps, _ := json.Marshal(in.AllowedApps)

	var version int
	err := r.pool.QueryRow(ctx, `
		update watch_rule wr
		set unlock_minutes = $3,
		    pass_threshold = $4,
		    subject_weight = $5::jsonb,
		    fun_ratio_cap = $6,
		    fun_enabled = $7,
		    weight_preset = $8,
		    time_window = $9::jsonb,
		    daily_limit = $10,
		    offline_fallback_min = $11,
		    scope_mode = $12,
		    allowed_apps = $13::jsonb,
		    timezone = $14,
		    version = version + 1,
		    updated_by = $2,
		    updated_at = now()
		where wr.child_id = $1
		  and exists (
		      select 1 from child_profile c
		      join family_member fm on fm.family_id = c.family_id
		      where c.id = wr.child_id and fm.account_id = $2
		  )
		returning version`,
		childID, accountID,
		in.UnlockMinutes, in.PassThreshold, string(weights), in.FunRatioCap, in.FunEnabled,
		in.WeightPreset, string(timeWindow), in.DailyLimit, in.OfflineFallbackMin,
		in.ScopeMode, string(apps), in.Timezone).Scan(&version)
	return version, err
}

func (r *repository) usageSeconds(ctx context.Context, childID string, start, end time.Time) (used, reserved int, err error) {
	if err = r.pool.QueryRow(ctx, `
		select coalesce(sum(watched_seconds), 0)::int
		from watch_log
		where child_id = $1 and start_at >= $2 and start_at < $3`,
		childID, start, end).Scan(&used); err != nil {
		return 0, 0, err
	}
	if err = r.pool.QueryRow(ctx, `
		select coalesce(sum(minutes * 60 - consumed_seconds), 0)::int
		from watch_grant
		where child_id = $1 and status = 'active' and valid_until > now()`,
		childID).Scan(&reserved); err != nil {
		return 0, 0, err
	}
	return used, reserved, nil
}

func (r *repository) validSubjects(ctx context.Context) (map[string]bool, error) {
	rows, err := r.pool.Query(ctx, `
		select code from tag_dict where tag_type = 'subject' and enabled = true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subjects := map[string]bool{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		subjects[code] = true
	}
	return subjects, rows.Err()
}

func (r *repository) audit(ctx context.Context, accountID, childID string, before, after Rule, ip string) error {
	encode := func(value Rule) string {
		payload, _ := json.Marshal(value)
		return string(payload)
	}
	_, err := r.pool.Exec(ctx, `
		insert into audit_log (actor_type, actor_id, action, target_type, target_id,
		                       before_data, after_data, ip)
		values ('parent', $1, 'rule.update', 'child', $2, $3::jsonb, $4::jsonb, nullif($5, '')::inet)`,
		accountID, childID, encode(before), encode(after), ip)
	return err
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
