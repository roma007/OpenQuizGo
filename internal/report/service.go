package report

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service 负责成长报告聚合与查询。
type Service struct {
	repo   *repository
	logger *slog.Logger
}

// New 构造报告服务。
func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{repo: newRepository(pool), logger: logger}
}

// List 返回孩子的周/月报告。
func (s *Service) List(ctx context.Context, accountID, childID, period string, limit int) ([]Report, error) {
	if period != "week" && period != "month" {
		return nil, newError(422, "VALIDATION_ERROR", "period 需为 week 或 month")
	}
	owns, err := s.repo.ownsChild(ctx, accountID, childID)
	if err != nil {
		return nil, err
	}
	if !owns {
		return nil, newError(404, "CHILD_NOT_FOUND", "孩子档案不存在")
	}
	if limit <= 0 || limit > 52 {
		limit = 8
	}
	reports, err := s.repo.list(ctx, accountID, childID, period, limit)
	if err != nil {
		return nil, err
	}
	timezone, err := s.repo.childTimezone(ctx, childID)
	if err != nil {
		timezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.UTC
	}
	for i := range reports {
		ps := reports[i].PeriodStart
		reports[i].PeriodStart = time.Date(ps.Year(), ps.Month(), ps.Day(), 0, 0, 0, 0, location)
		reports[i].PeriodEnd = periodEnd(reports[i].Period, reports[i].PeriodStart)
	}
	return reports, nil
}

// Generate 聚合指定周期的报告（幂等）。
func (s *Service) Generate(ctx context.Context, childID, period string, start time.Time) (Report, error) {
	if period != "week" && period != "month" {
		return Report{}, newError(422, "VALIDATION_ERROR", "period 需为 week 或 month")
	}
	timezone, err := s.repo.childTimezone(ctx, childID)
	if err != nil {
		if isNoRows(err) {
			return Report{}, newError(404, "CHILD_NOT_FOUND", "孩子档案不存在")
		}
		return Report{}, err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.UTC
	}
	start = periodStart(period, start.In(location), location)
	end := periodEnd(period, start)

	rows, err := s.repo.aggregateAnswers(ctx, childID, start, end, timezone)
	if err != nil {
		return Report{}, err
	}
	subjects := map[string]SubjectStat{}
	fun := SubjectStat{}
	for _, row := range rows {
		if row.Subject == "fun" {
			fun.Answered += row.Answered
			fun.Correct += row.Correct
			fun.Trend = append(fun.Trend, DailyStat{Date: row.Date, Answered: row.Answered, Correct: row.Correct})
			continue
		}
		current := subjects[row.Subject]
		current.Answered += row.Answered
		current.Correct += row.Correct
		current.Trend = append(current.Trend, DailyStat{Date: row.Date, Answered: row.Answered, Correct: row.Correct})
		subjects[row.Subject] = current
	}
	for subject, stat := range subjects {
		stat.Accuracy = accuracy(stat.Answered, stat.Correct)
		sort.Slice(stat.Trend, func(i, j int) bool { return stat.Trend[i].Date < stat.Trend[j].Date })
		subjects[subject] = stat
	}
	fun.Accuracy = accuracy(fun.Answered, fun.Correct)
	sort.Slice(fun.Trend, func(i, j int) bool { return fun.Trend[i].Date < fun.Trend[j].Date })

	watchMinutes, err := s.repo.watchMinutes(ctx, childID, start, end)
	if err != nil {
		return Report{}, err
	}
	payload := make(map[string]any, len(subjects))
	for subject, stat := range subjects {
		payload[subject] = stat
	}
	funPayload := map[string]any{
		"answered": fun.Answered,
		"correct":  fun.Correct,
		"accuracy": fun.Accuracy,
		"trend":    fun.Trend,
	}
	id, generatedAt, err := s.repo.upsert(ctx, childID, period, start, payload, funPayload, watchMinutes)
	if err != nil {
		return Report{}, err
	}
	return Report{
		ID:           id,
		ChildID:      childID,
		Period:       period,
		PeriodStart:  start,
		PeriodEnd:    end,
		SubjectStats: subjects,
		FunStats:     fun,
		WatchMinutes: watchMinutes,
		GeneratedAt:  generatedAt,
	}, nil
}

// GenerateAll 为所有孩子生成当前与前一周/月报告。
func (s *Service) GenerateAll(ctx context.Context) (int, error) {
	children, err := s.repo.children(ctx)
	if err != nil {
		return 0, err
	}
	generated := 0
	for _, child := range children {
		location, err := time.LoadLocation(child.Timezone)
		if err != nil {
			location = time.UTC
		}
		now := time.Now().In(location)
		targets := []struct {
			period string
			start  time.Time
		}{
			{"week", periodStart("week", now, location)},
			{"week", periodStart("week", now.AddDate(0, 0, -7), location)},
			{"month", periodStart("month", now, location)},
			{"month", periodStart("month", now.AddDate(0, -1, 0), location)},
		}
		for _, target := range targets {
			if _, err := s.Generate(ctx, child.ID, target.period, target.start); err != nil {
				s.logger.Error("generate report", "error", err, "child_id", child.ID,
					"period", target.period, "start", target.start)
				continue
			}
			generated++
		}
	}
	return generated, nil
}

func periodStart(period string, at time.Time, location *time.Location) time.Time {
	local := at.In(location)
	switch period {
	case "week":
		offset := (int(local.Weekday()) + 6) % 7 // Monday = 0
		return time.Date(local.Year(), local.Month(), local.Day()-offset, 0, 0, 0, 0, location)
	default:
		return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
	}
}

func periodEnd(period string, start time.Time) time.Time {
	if period == "week" {
		return start.AddDate(0, 0, 7)
	}
	return start.AddDate(0, 1, 0)
}

func accuracy(answered, correct int) float64 {
	if answered == 0 {
		return 0
	}
	return math.Round(float64(correct)/float64(answered)*1000) / 1000
}
