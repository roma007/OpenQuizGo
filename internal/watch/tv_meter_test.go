package watch

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/rule"
)

// TestMeterTVWatch 覆盖 081：做题开关关闭时电视没有答题授权，
// 观看时长改由 /v1/tv/meter 计量，额度耗尽后 context 判定硬停。
func TestMeterTVWatch(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	prepareWatchDB(t, dbURL)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	childID := seedWatchChild(t, pool)
	var familyID string
	if err := pool.QueryRow(ctx,
		`select family_id from child_profile where id = $1`, childID).Scan(&familyID); err != nil {
		t.Fatalf("family: %v", err)
	}
	var devicePK string
	if err := pool.QueryRow(ctx, `
		insert into tv_device (family_id, child_id, device_id, name)
		values ($1, $2, 'tv-meter', '计量电视') returning id`,
		familyID, childID).Scan(&devicePK); err != nil {
		t.Fatalf("tv device: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`insert into tv_config (device_pk) values ($1)`, devicePK); err != nil {
		t.Fatalf("tv config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ruleSvc := rule.New(pool, nil, logger)
	svc, err := New(pool, ruleSvc, nil, "test-secret-0123456789-0123456789", "", logger)
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.DevicePrincipal{
		Kind:     "tv",
		DevicePK: devicePK,
		DeviceID: "tv-meter",
		ChildID:  childID,
	}

	// 做题开关默认开启：计量接口拒绝，避免绕过答题心跳。
	if _, err := svc.MeterTVWatch(ctx, principal, 60, 0); err == nil {
		t.Fatal("expected QUIZ_ENABLED when quiz is on")
	} else if watchErr, ok := err.(*Error); !ok || watchErr.Code != "QUIZ_ENABLED" {
		t.Fatalf("unexpected error: %v", err)
	}

	// 关掉做题、上限设为 2 分钟，再看 30 秒 + 90 秒即耗尽。
	if _, err := pool.Exec(ctx, `
		update tv_config set quiz_enabled = false, daily_limit = 2 where device_pk = $1`,
		devicePK); err != nil {
		t.Fatalf("update config: %v", err)
	}
	result, err := svc.MeterTVWatch(ctx, principal, 30, 0)
	if err != nil {
		t.Fatalf("meter: %v", err)
	}
	if result.HardStop || result.RemainingMinutes != 1 {
		t.Fatalf("expected 1 minute left, not hard stopped: %+v", result)
	}
	result, err = svc.MeterTVWatch(ctx, principal, 90, 0)
	if err != nil {
		t.Fatalf("meter second: %v", err)
	}
	if !result.HardStop || result.Allowed || result.Reason != "daily_limit" {
		t.Fatalf("expected hard stop after quota exhausted: %+v", result)
	}

	// 非电视设备（旧激活码）不能走这个接口。
	legacy := identity.DevicePrincipal{DeviceID: "tv-meter", ChildID: childID}
	if _, err := svc.MeterTVWatch(ctx, legacy, 10, 0); err == nil {
		t.Fatal("expected non-tv principal to be rejected")
	}
}
