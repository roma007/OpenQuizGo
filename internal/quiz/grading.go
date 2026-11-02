package quiz

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

var errInvalidAnswer = errors.New("答案格式不合法")

// grade 使用会话快照判分，支持选择/语音/看图/填空/排序/连线。
func grade(snapshot itemSnapshot, given json.RawMessage) (bool, error) {
	if len(given) == 0 || string(given) == "null" {
		return false, errInvalidAnswer
	}
	switch snapshot.Type {
	case "choice", "image":
		return equalStringSet(snapshot.Answer, given)
	case "sort":
		return equalStringSequence(snapshot.Answer, given)
	case "fill", "voice":
		return matchText(snapshot.Answer, snapshot.Alias, given)
	case "match":
		return equalStringMap(snapshot.Answer, given)
	default:
		return false, errInvalidAnswer
	}
}

func equalStringSet(expected, given json.RawMessage) (bool, error) {
	want, ok1 := stringList(expected)
	got, ok2 := stringList(given)
	if !ok1 || !ok2 || len(want) != len(got) {
		return false, errInvalidAnswer
	}
	want = append([]string(nil), want...)
	got = append([]string(nil), got...)
	sort.Strings(want)
	sort.Strings(got)
	for i := range want {
		if want[i] != got[i] {
			return false, nil
		}
	}
	return true, nil
}

func equalStringSequence(expected, given json.RawMessage) (bool, error) {
	want, ok1 := stringList(expected)
	got, ok2 := stringList(given)
	if !ok1 || !ok2 || len(want) != len(got) {
		return false, errInvalidAnswer
	}
	for i := range want {
		if want[i] != got[i] {
			return false, nil
		}
	}
	return true, nil
}

func matchText(expected, alias, given json.RawMessage) (bool, error) {
	want, ok1 := stringList(expected)
	got, ok2 := stringList(given)
	if !ok1 || !ok2 || len(got) == 0 {
		return false, errInvalidAnswer
	}
	if len(want) == 1 {
		if normalizeText(got[0]) == normalizeText(want[0]) {
			return true, nil
		}
		aliases, _ := stringList(alias)
		for _, item := range aliases {
			if normalizeText(got[0]) == normalizeText(item) {
				return true, nil
			}
		}
		return false, nil
	}
	if len(want) != len(got) {
		return false, errInvalidAnswer
	}
	for i := range want {
		if normalizeText(want[i]) != normalizeText(got[i]) {
			return false, nil
		}
	}
	return true, nil
}

func equalStringMap(expected, given json.RawMessage) (bool, error) {
	var want, got map[string]string
	if json.Unmarshal(expected, &want) != nil || json.Unmarshal(given, &got) != nil {
		return false, errInvalidAnswer
	}
	if len(want) != len(got) {
		return false, nil
	}
	for key, value := range want {
		if got[key] != value {
			return false, nil
		}
	}
	return true, nil
}

func stringList(raw json.RawMessage) ([]string, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, true
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, true
	}
	return nil, false
}

func normalizeText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), ""))
}
