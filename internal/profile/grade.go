package profile

import "time"

// Stage 把年级标签映射到题库档位；题库目前只有 preschool/low/mid/high 四档。
func Stage(gradeTag string) string {
	switch gradeTag {
	case "":
		return "preschool"
	case "g1", "g2":
		return "low"
	case "g3", "g4":
		return "mid"
	case "g5", "g6", "g7", "g8", "g9":
		return "high"
	default:
		return "low"
	}
}

// Effective 返回自动升年级后的有效年级标签。
//
// 规则：以 gradeSetAt 为基准，每跨过一个 9 月 1 日升一级，封顶 g9。
// 读时计算，不依赖定时任务。
func Effective(gradeTag string, gradeSetAt *time.Time, now time.Time) string {
	if gradeTag == "" || gradeSetAt == nil {
		return gradeTag
	}
	base := gradeNumber(gradeTag)
	if base <= 0 {
		return gradeTag
	}
	set := *gradeSetAt
	if set.After(now) {
		return gradeTag
	}
	promotions := boundaries(set, now)
	grade := base + promotions
	if grade > 9 {
		grade = 9
	}
	return "g" + itoa(grade)
}

// boundaries 统计 (from, to] 之间跨过的 9 月 1 日个数。
func boundaries(from, to time.Time) int {
	start := time.Date(from.Year(), time.September, 1, 0, 0, 0, 0, from.Location())
	if start.Before(from) || start.Equal(from) {
		start = start.AddDate(1, 0, 0)
	}
	count := 0
	for !start.After(to) {
		count++
		start = start.AddDate(1, 0, 0)
	}
	return count
}

func gradeNumber(tag string) int {
	if len(tag) != 2 || tag[0] != 'g' {
		return 0
	}
	n := int(tag[1] - '0')
	if n < 1 || n > 9 {
		return 0
	}
	return n
}

func itoa(n int) string {
	return string(rune('0' + n))
}
