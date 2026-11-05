package question

import (
	"encoding/json"
	"testing"
)

// choiceInput 构造一个不依赖数据库的选择题校验入参。
func choiceInput(t *testing.T, optionsJSON, answerJSON string) *Input {
	t.Helper()
	return &Input{
		Subject:    "math",
		Category:   "core",
		Stage:      "low",
		Difficulty: 2,
		Type:       "choice",
		Stem:       "看图选择",
		Options:    json.RawMessage(optionsJSON),
		Answer:     json.RawMessage(answerJSON),
	}
}

func testTagSets() tagSets {
	return tagSets{
		subjects:   map[string]bool{"math": true},
		categories: map[string]bool{"core": true},
		stages:     map[string]bool{"low": true},
		types:      map[string]bool{"choice": true},
	}
}

func optionProblems(t *testing.T, optionsJSON, answerJSON string) []ImportError {
	t.Helper()
	return validate(choiceInput(t, optionsJSON, answerJSON), testTagSets(), nil)
}

func TestValidateOptionImages(t *testing.T) {
	cases := []struct {
		name    string
		options string
		answer  string
		wantErr bool
	}{
		{
			name: "two images valid",
			options: `[
				{"id":"a","text":"","images":["/v1/media/a.png","/v1/media/b.png"]},
				{"id":"b","text":"B"}
			]`,
			answer: `["a"]`,
		},
		{
			name: "four images rejected with 422",
			options: `[
				{"id":"a","text":"","images":["/v1/media/a.png","/v1/media/b.png","/v1/media/c.png","/v1/media/d.png"]},
				{"id":"b","text":"B"}
			]`,
			answer:  `["a"]`,
			wantErr: true,
		},
		{
			name: "legacy image only valid",
			options: `[
				{"id":"a","text":"","image":"/v1/media/a.png"},
				{"id":"b","text":"B"}
			]`,
			answer: `["a"]`,
		},
		{
			name: "empty option rejected",
			options: `[
				{"id":"a","text":""},
				{"id":"b","text":"B"}
			]`,
			answer:  `["a"]`,
			wantErr: true,
		},
		{
			name: "blank image url rejected",
			options: `[
				{"id":"a","text":"","images":["/v1/media/a.png"," "]},
				{"id":"b","text":"B"}
			]`,
			answer:  `["a"]`,
			wantErr: true,
		},
		{
			name: "duplicate id rejected",
			options: `[
				{"id":"a","text":"A"},
				{"id":"a","text":"B"}
			]`,
			answer:  `["a"]`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := optionProblems(t, tc.options, tc.answer)
			if tc.wantErr && len(problems) == 0 {
				t.Fatal("expected validation problem, got none")
			}
			if !tc.wantErr && len(problems) != 0 {
				t.Fatalf("unexpected validation problem: %+v", problems[0])
			}
			if tc.wantErr && problems[0].Field != "options" {
				t.Fatalf("problem field = %q, want options", problems[0].Field)
			}
		})
	}
}
