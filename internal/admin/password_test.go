package admin

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestValidateNewPassword(t *testing.T) {
	cases := []struct {
		name     string
		next     string
		current  string
		username string
		want     string
	}{
		{"ok", "StrongPass!2026", "old-password-123", "admin", ""},
		{"ok-cjk", strings.Repeat("密", 12), "old-password-123", "admin", ""},
		{"too short", "Short11", "old-password-123", "admin", "新密码至少 12 个字符"},
		{"too long", strings.Repeat("a", 73), "old-password-123", "admin", "新密码不能超过 72 字节"},
		{"same as current", "old-password-123", "old-password-123", "admin", "新密码不能与当前密码相同"},
		{"same as username", "Administrator", "old-password-123", "administrator", "新密码不能与用户名相同"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateNewPassword(tc.next, tc.current, tc.username); got != tc.want {
				t.Fatalf("validateNewPassword() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTokenEpochRoundTrip(t *testing.T) {
	s := newTestService(time.Hour)
	token, err := s.sign(adminRecord{ID: "admin-1", Username: "admin"},
		[]string{"super_admin"}, []string{"*"}, time.Now().Add(time.Hour), 7)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	principal, err := s.parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if principal.Epoch != 7 {
		t.Fatalf("epoch = %d, want 7", principal.Epoch)
	}

	// 未配置 Redis（单测环境）时不校验版本，避免误伤。
	if !s.tokenFresh(context.Background(), principal) {
		t.Error("tokenFresh() = false without redis, want true")
	}
}
