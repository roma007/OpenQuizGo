package quiz

import "testing"

func TestSessionPassedByCorrectCount(t *testing.T) {
	cases := []struct {
		correct int
		need    int
		want    bool
	}{
		{0, 2, false},
		{1, 2, false},
		{2, 2, true},
		{5, 2, true},
		{1, 0, true}, // 未配置时至少需要 1 题
		{0, 0, false},
	}
	for _, tc := range cases {
		if got := sessionPassed(tc.correct, tc.need); got != tc.want {
			t.Fatalf("sessionPassed(%d,%d)=%v want %v", tc.correct, tc.need, got, tc.want)
		}
	}
}
