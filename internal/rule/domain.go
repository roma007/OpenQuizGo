package rule

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// MaxSessionQuestions 是单次会话的题量上限。
const MaxSessionQuestions = 12

// Window 是允许使用的时段；start > end 表示跨零点。
type Window struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// TimeWindow 是规则中的时段配置；allow 为空表示不限时段。
type TimeWindow struct {
	Allow []Window `json:"allow"`
}

// Rule 是孩子观看规则。
type Rule struct {
	UnlockMinutes      int                `json:"unlock_minutes"`
	PassThreshold      float64            `json:"pass_threshold"`
	SubjectWeight      map[string]float64 `json:"subject_weight"`
	FunRatioCap        int                `json:"fun_ratio_cap"`
	FunEnabled         bool               `json:"fun_enabled"`
	WeightPreset       string             `json:"weight_preset,omitempty"`
	TimeWindow         TimeWindow         `json:"time_window"`
	DailyLimit         int                `json:"daily_limit"`
	OfflineFallbackMin int                `json:"offline_fallback_min"`
	ScopeMode          string             `json:"scope_mode"`
	AllowedApps        []string           `json:"allowed_apps"`
	Timezone           string             `json:"timezone"`
}

// ValidationError 是规则校验错误。
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

func invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

var (
	clockPattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
	validScopes  = map[string]bool{"media": true, "custom": true, "all_apps": true}
	validPresets = map[string]bool{"balanced": true, "math": true, "literacy": true, "custom": true}
)

// Validate 校验规则完整性与可达性。
func Validate(r Rule) error {
	if r.UnlockMinutes < 1 || r.UnlockMinutes > 240 {
		return invalid("unlock_minutes", "解锁时长需在 1–240 分钟")
	}
	if r.PassThreshold <= 0 {
		return invalid("pass_threshold", "加权门槛必须大于 0")
	}
	if len(r.SubjectWeight) == 0 {
		return invalid("subject_weight", "学科权重不能为空")
	}

	minSubjectWeight := math.MaxFloat64
	hasSubject := false
	for subject, weight := range r.SubjectWeight {
		if weight < 0 {
			return invalid("subject_weight", "学科权重不能为负")
		}
		if subject == "fun" {
			continue
		}
		if weight > 0 {
			hasSubject = true
			if weight < minSubjectWeight {
				minSubjectWeight = weight
			}
		}
	}
	if !hasSubject {
		return invalid("subject_weight", "至少需要一个非趣味学科权重")
	}
	if funWeight := r.SubjectWeight["fun"]; funWeight > 0 && funWeight >= minSubjectWeight {
		return invalid("subject_weight", "趣味题权重必须低于最低学科权重")
	}
	if r.FunRatioCap < 0 || r.FunRatioCap > 10 {
		return invalid("fun_ratio_cap", "趣味题上限需在 0–10 之间")
	}
	if r.DailyLimit == 0 {
		r.DailyLimit = 60
	}
	if r.DailyLimit < 1 || r.DailyLimit > 720 {
		return invalid("daily_limit", "每日上限需在 1–720 分钟")
	}
	if r.OfflineFallbackMin < 0 || r.OfflineFallbackMin > 120 {
		return invalid("offline_fallback_min", "离线降级需在 0–120 分钟")
	}
	if r.ScopeMode != "" && !validScopes[r.ScopeMode] {
		return invalid("scope_mode", "管控模式不合法")
	}
	if r.Timezone == "" {
		r.Timezone = "Asia/Shanghai"
	}
	if _, err := time.LoadLocation(r.Timezone); err != nil {
		return invalid("timezone", "时区不合法")
	}
	if len(r.TimeWindow.Allow) > 4 {
		return invalid("time_window", "最多配置 4 个时段")
	}
	for _, window := range r.TimeWindow.Allow {
		if !clockPattern.MatchString(window.Start) || !clockPattern.MatchString(window.End) {
			return invalid("time_window", "时段格式需为 HH:MM")
		}
		if window.Start == window.End {
			return invalid("time_window", "时段起止不能相同")
		}
	}
	if len(r.AllowedApps) > 50 {
		return invalid("allowed_apps", "应用清单最多 50 个")
	}

	n, err := SessionSize(r)
	if err != nil {
		return invalid("pass_threshold", err.Error())
	}
	maxScore := theoreticalMax(r, n)
	if r.PassThreshold > maxScore+1e-9 {
		return invalid("pass_threshold",
			fmt.Sprintf("门槛 %.1f 高于当前题量可达的加权分 %.1f", r.PassThreshold, maxScore))
	}
	return nil
}

// SessionSize 按加权门槛推导会话题量。
func SessionSize(r Rule) (int, error) {
	maxWeight := 0.0
	for subject, weight := range r.SubjectWeight {
		if subject == "fun" {
			continue
		}
		if weight > maxWeight {
			maxWeight = weight
		}
	}
	if maxWeight <= 0 {
		return 0, errors.New("至少需要一个非趣味学科权重")
	}
	nMin := int(math.Ceil(r.PassThreshold / maxWeight))
	if nMin < 3 {
		nMin = 3
	}
	n := nMin + 2
	if n > MaxSessionQuestions {
		n = MaxSessionQuestions
	}
	if n < 3 {
		n = 3
	}
	return n, nil
}

func theoreticalMax(r Rule, n int) float64 {
	weights := make(map[string]float64)
	for subject, weight := range r.SubjectWeight {
		if subject != "fun" && weight > 0 {
			weights[subject] = weight
		}
	}
	funSlots := 0
	if r.FunEnabled && r.FunRatioCap > 0 && r.SubjectWeight["fun"] > 0 {
		funSlots = min(r.FunRatioCap, max(1, n-1))
	}
	quota := Allocate(weights, n-funSlots)
	score := 0.0
	for subject, count := range quota {
		score += float64(count) * r.SubjectWeight[subject]
	}
	score += float64(funSlots) * r.SubjectWeight["fun"]
	return score
}

// Allocate 使用最大余数法分配题量。
func Allocate(weights map[string]float64, total int) map[string]int {
	quota := map[string]int{}
	if total <= 0 {
		return quota
	}
	subjects := SubjectsByWeight(weights)
	sum := 0.0
	for _, subject := range subjects {
		sum += weights[subject]
	}
	if sum <= 0 {
		return quota
	}
	type remainder struct {
		subject string
		value   float64
	}
	remainders := make([]remainder, 0, len(subjects))
	allocated := 0
	for _, subject := range subjects {
		exact := float64(total) * weights[subject] / sum
		base := int(math.Floor(exact))
		quota[subject] = base
		allocated += base
		remainders = append(remainders, remainder{subject, exact - float64(base)})
	}
	sort.SliceStable(remainders, func(i, j int) bool {
		if remainders[i].value != remainders[j].value {
			return remainders[i].value > remainders[j].value
		}
		return weights[remainders[i].subject] > weights[remainders[j].subject]
	})
	for i := 0; allocated < total; i++ {
		quota[remainders[i%len(remainders)].subject]++
		allocated++
	}
	return quota
}

// SubjectsByWeight 按权重降序返回启用学科。
func SubjectsByWeight(weights map[string]float64) []string {
	subjects := make([]string, 0, len(weights))
	for subject, weight := range weights {
		if weight > 0 {
			subjects = append(subjects, subject)
		}
	}
	sort.Slice(subjects, func(i, j int) bool {
		if weights[subjects[i]] != weights[subjects[j]] {
			return weights[subjects[i]] > weights[subjects[j]]
		}
		return subjects[i] < subjects[j]
	})
	return subjects
}

// AllowedAt 判断指定时间是否在允许时段内；未配置时段视为不限。
func AllowedAt(r Rule, at time.Time) (bool, error) {
	if len(r.TimeWindow.Allow) == 0 {
		return true, nil
	}
	location, err := time.LoadLocation(r.Timezone)
	if err != nil {
		return false, err
	}
	local := at.In(location)
	minutes := local.Hour()*60 + local.Minute()
	for _, window := range r.TimeWindow.Allow {
		start := clockMinutes(window.Start)
		end := clockMinutes(window.End)
		if start < end {
			if minutes >= start && minutes < end {
				return true, nil
			}
		} else if minutes >= start || minutes < end {
			return true, nil
		}
	}
	return false, nil
}

// DayBounds 返回规则时区下当天的起止时间。
func DayBounds(r Rule, at time.Time) (time.Time, time.Time, error) {
	location, err := time.LoadLocation(r.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	local := at.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return start, start.Add(24 * time.Hour), nil
}

func clockMinutes(value string) int {
	return int(value[0]-'0')*600 + int(value[1]-'0')*60 + int(value[3]-'0')*10 + int(value[4]-'0')
}

// Normalize 填充默认值并清理空白。
func Normalize(r Rule) Rule {
	if r.Timezone == "" {
		r.Timezone = "Asia/Shanghai"
	}
	if r.ScopeMode == "" {
		r.ScopeMode = "media"
	}
	if r.WeightPreset == "" {
		r.WeightPreset = "custom"
	}
	if r.DailyLimit == 0 {
		r.DailyLimit = 60
	}
	for subject, weight := range r.SubjectWeight {
		if weight < 0 {
			r.SubjectWeight[subject] = 0
		}
	}
	apps := make([]string, 0, len(r.AllowedApps))
	for _, app := range r.AllowedApps {
		app = strings.TrimSpace(app)
		if app != "" {
			apps = append(apps, app)
		}
	}
	r.AllowedApps = apps
	return r
}

// Presets 是家长端预设权重。
func Presets() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"balanced": {
			"math": 1.5, "chinese": 1.5, "english": 1.5,
			"science": 1, "logic": 1, "safety": 1, "fun": 0.5,
		},
		"math": {
			"math": 2, "chinese": 1.2, "english": 1.2,
			"science": 1, "logic": 1, "safety": 1, "fun": 0.5,
		},
		"literacy": {
			"math": 1.2, "chinese": 1.2, "english": 1,
			"science": 2, "logic": 2, "safety": 2, "fun": 0.5,
		},
	}
}
