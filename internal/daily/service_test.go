package daily

import "testing"

func TestEqualSet(t *testing.T) {
	cases := []struct {
		name string
		want string
		got  string
		ok   bool
	}{
		{"same set", `["a","b"]`, `["b","a"]`, true},
		{"single", `["a"]`, `"a"`, true},
		{"different", `["a"]`, `["b"]`, false},
		{"length", `["a"]`, `["a","b"]`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := equalSet([]byte(tc.want), []byte(tc.got)); got != tc.ok {
				t.Fatalf("equalSet = %v, want %v", got, tc.ok)
			}
		})
	}
}
