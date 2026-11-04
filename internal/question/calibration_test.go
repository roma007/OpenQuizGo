package question

import "testing"

func TestSuggestDifficulty(t *testing.T) {
	cases := []struct {
		name       string
		difficulty int
		answered   int
		correct    int
		want       int
	}{
		{"high accuracy raises", 2, 20, 18, 3},
		{"max difficulty stays", 5, 20, 20, 5},
		{"low accuracy lowers", 3, 20, 6, 2},
		{"min difficulty stays", 1, 20, 0, 1},
		{"middle keeps", 3, 20, 12, 3},
		{"no answers keeps", 2, 0, 0, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := suggestDifficulty(tc.difficulty, tc.answered, tc.correct)
			if got != tc.want {
				t.Fatalf("suggest = %d, want %d", got, tc.want)
			}
		})
	}
}
