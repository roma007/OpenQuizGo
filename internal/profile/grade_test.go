package profile

import (
	"testing"
	"time"
)

func TestStage(t *testing.T) {
	cases := map[string]string{
		"":   "preschool",
		"g1": "low",
		"g2": "low",
		"g3": "mid",
		"g4": "mid",
		"g5": "high",
		"g9": "high",
		"x1": "low",
	}
	for grade, want := range cases {
		if got := Stage(grade); got != want {
			t.Errorf("Stage(%q) = %q, want %q", grade, got, want)
		}
	}
}

func TestEffectiveAutoPromotion(t *testing.T) {
	set := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		tag  string
		now  time.Time
		want string
	}{
		{"设年级当天不升级", "g1", set, "g1"},
		{"8月31日仍是 g1", "g1", time.Date(2026, time.August, 31, 23, 59, 0, 0, time.UTC), "g1"},
		{"9月1日升到 g2", "g1", time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC), "g2"},
		{"跨两年升两级", "g3", time.Date(2028, time.September, 2, 0, 0, 0, 0, time.UTC), "g6"},
		{"封顶 g9", "g9", time.Date(2030, time.September, 1, 0, 0, 0, 0, time.UTC), "g9"},
		{"超过 g9 仍封顶", "g8", time.Date(2031, time.September, 1, 0, 0, 0, 0, time.UTC), "g9"},
		{"未设年级不升级", "", set, ""},
		{"非法标签原样返回", "abc", time.Date(2031, time.September, 1, 0, 0, 0, 0, time.UTC), "abc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var at *time.Time
			if tc.tag != "" && tc.tag[0] == 'g' {
				at = &set
			}
			if got := Effective(tc.tag, at, tc.now); got != tc.want {
				t.Errorf("Effective(%q, %v, %v) = %q, want %q",
					tc.tag, at, tc.now, got, tc.want)
			}
		})
	}
}

func TestEffectiveWithoutSetAt(t *testing.T) {
	if got := Effective("g4", nil, time.Now()); got != "g4" {
		t.Fatalf("Effective without grade_set_at = %q, want g4", got)
	}
}
