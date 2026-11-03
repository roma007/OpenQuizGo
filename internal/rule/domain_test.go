package rule

import (
	"testing"
	"time"
)

func TestAllocate(t *testing.T) {
	quota := Allocate(map[string]float64{"math": 2, "chinese": 1, "science": 1}, 6)
	if quota["math"] != 3 || quota["chinese"] != 2 || quota["science"] != 1 {
		t.Fatalf("unexpected quota: %+v", quota)
	}
	total := 0
	for _, count := range quota {
		total += count
	}
	if total != 6 {
		t.Fatalf("total = %d, want 6", total)
	}
}

func TestSessionSize(t *testing.T) {
	n, err := SessionSize(Rule{PassThreshold: 3, SubjectWeight: map[string]float64{"math": 2, "chinese": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("n = %d, want 5", n)
	}
}

func TestValidateRejectsFunWeight(t *testing.T) {
	err := Validate(Rule{
		UnlockMinutes: 20,
		PassThreshold: 3,
		SubjectWeight: map[string]float64{"math": 1, "fun": 1},
		DailyLimit:    60,
		Timezone:      "Asia/Shanghai",
	})
	if err == nil {
		t.Fatal("expected error when fun weight >= subject weight")
	}
}

func TestValidateRejectsUnreachableThreshold(t *testing.T) {
	err := Validate(Rule{
		UnlockMinutes: 20,
		PassThreshold: 100,
		SubjectWeight: map[string]float64{"math": 2},
		DailyLimit:    60,
		Timezone:      "Asia/Shanghai",
	})
	if err == nil {
		t.Fatal("expected unreachable threshold error")
	}
}

func TestAllowedAt(t *testing.T) {
	r := Rule{
		Timezone: "Asia/Shanghai",
		TimeWindow: TimeWindow{Allow: []Window{
			{Start: "07:00", End: "12:00"},
			{Start: "20:00", End: "07:00"},
		}},
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	cases := []struct {
		at   time.Time
		want bool
	}{
		{time.Date(2026, 11, 3, 8, 0, 0, 0, loc), true},
		{time.Date(2026, 11, 3, 13, 0, 0, 0, loc), false},
		{time.Date(2026, 11, 3, 21, 0, 0, 0, loc), true},
		{time.Date(2026, 11, 3, 6, 30, 0, 0, loc), true},
	}
	for _, tc := range cases {
		got, err := AllowedAt(r, tc.at)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("AllowedAt(%s) = %v, want %v", tc.at.Format("15:04"), got, tc.want)
		}
	}
}
