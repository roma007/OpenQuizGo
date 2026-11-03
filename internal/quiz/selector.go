package quiz

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/roma007/OpenQuizGo/internal/rule"
	"github.com/roma007/OpenQuizGo/internal/watch"
)

// Rule 是规则子集别名，统一由 rule 包校验。
type Rule = rule.Rule

const defaultPoolLimit = 2000

var (
	stageRanks = map[string]int{"preschool": 0, "low": 1, "mid": 2, "high": 3}
	stageOrder = map[string][]string{
		"preschool": {"preschool", "low", "mid", "high"},
		"low":       {"low", "preschool", "mid", "high"},
		"mid":       {"mid", "low", "high", "preschool"},
		"high":      {"high", "mid", "low", "preschool"},
	}
	validStages = map[string]bool{"preschool": true, "low": true, "mid": true, "high": true}
)

// Service 是组卷服务。
type Service struct {
	repo     *repository
	redis    *redis.Client
	ruleSvc  *rule.Service
	watchSvc *watch.Service
	logger   *slog.Logger
}

// New 构造组卷服务。
func New(pool *pgxpool.Pool, rdb *redis.Client, ruleSvc *rule.Service, watchSvc *watch.Service, logger *slog.Logger) *Service {
	return &Service{
		repo:     newRepository(pool),
		redis:    rdb,
		ruleSvc:  ruleSvc,
		watchSvc: watchSvc,
		logger:   logger,
	}
}

// Select 按规则组卷。
func (s *Service) Select(ctx context.Context, req SelectRequest) (Result, error) {
	stage, grade := req.Stage, req.GradeTag
	if req.ChildID != "" {
		childStage, childGrade, err := s.repo.childStage(ctx, req.ChildID)
		if err != nil {
			return Result{}, newError(404, "CHILD_NOT_FOUND", "孩子档案不存在")
		}
		stage, grade = childStage, childGrade
	}
	stage = strings.TrimSpace(stage)
	if !validStages[stage] {
		return Result{}, newError(422, "VALIDATION_ERROR", "学段不合法")
	}
	if err := rule.Validate(req.Rule); err != nil {
		return Result{}, newError(422, "VALIDATION_ERROR", err.Error())
	}

	seed := req.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	stats, err := s.repo.difficultyStats(ctx, req.ChildID)
	if err != nil {
		return Result{}, err
	}
	target := targetDifficulty(stage, grade, stats)

	n, err := rule.SessionSize(req.Rule)
	if err != nil {
		return Result{}, newError(422, "VALIDATION_ERROR", err.Error())
	}
	if req.MaxQuestions > 0 && req.MaxQuestions < n {
		n = max(3, req.MaxQuestions)
	}

	subjectWeights := make(map[string]float64)
	for subject, weight := range req.Rule.SubjectWeight {
		if subject == "fun" || weight <= 0 {
			continue
		}
		subjectWeights[subject] = weight
	}
	if len(subjectWeights) == 0 {
		return Result{}, newError(422, "VALIDATION_ERROR", "至少需要一个学科权重")
	}

	funSlots := 0
	if req.Rule.FunEnabled && req.Rule.FunRatioCap > 0 && req.Rule.SubjectWeight["fun"] > 0 {
		funSlots = min(req.Rule.FunRatioCap, max(1, n-1))
	}
	subjectSlots := n - funSlots
	quota := rule.Allocate(subjectWeights, subjectSlots)

	selected := make([]Selected, 0, n)
	chosen := map[string]bool{}
	warnings := make([]string, 0)
	addWarning := func(w string) {
		for _, existing := range warnings {
			if existing == w {
				return
			}
		}
		warnings = append(warnings, w)
	}

	stages := stageOrder[stage]
	primaryRank := stageRanks[stage]

	for _, subject := range sortedKeys(quota) {
		if err := s.fillSubject(ctx, req.ChildID, subject, stages, primaryRank,
			target, quota[subject], rng, chosen, &selected, addWarning); err != nil {
			return Result{}, err
		}
	}

	if funSlots > 0 {
		funStages := []string{"preschool", "low"}
		if stage == "mid" || stage == "high" {
			funStages = []string{"mid", "high"}
		}
		if err := s.fillSubject(ctx, req.ChildID, "fun", funStages, -1,
			target, funSlots, rng, chosen, &selected, addWarning); err != nil {
			return Result{}, err
		}
	}

	// 配额未满时，按权重从高到低再分配。
	if len(selected) < n {
		ordered := rule.SubjectsByWeight(subjectWeights)
		for pass := 0; pass < 2 && len(selected) < n; pass++ {
			progress := false
			for _, subject := range ordered {
				if len(selected) >= n {
					break
				}
				need := n - len(selected)
				before := len(selected)
				if err := s.fillSubject(ctx, req.ChildID, subject, stages, primaryRank,
					target, need, rng, chosen, &selected, addWarning); err != nil {
					return Result{}, err
				}
				if len(selected) > before {
					progress = true
				}
			}
			if !progress {
				break
			}
			addWarning("quota_redistributed")
		}
	}

	rng.Shuffle(len(selected), func(i, j int) {
		selected[i], selected[j] = selected[j], selected[i]
	})

	score := 0.0
	actual := map[string]int{}
	for _, item := range selected {
		score += req.Rule.SubjectWeight[item.Subject]
		actual[item.Subject]++
	}

	result := Result{
		Total:            len(selected),
		Quota:            actual,
		Selected:         selected,
		MaxScore:         round2(score),
		DifficultyTarget: target,
		Warnings:         warnings,
		Seed:             seed,
	}
	if len(selected) < 3 || score < req.Rule.PassThreshold {
		apiErr := newError(409, "INSUFFICIENT_POOL",
			fmt.Sprintf("题池不足：已选 %d 题，最高加权分 %.1f，低于门槛 %.1f", len(selected), score, req.Rule.PassThreshold))
		apiErr.Warnings = warnings
		return result, apiErr
	}
	return result, nil
}

func (s *Service) fillSubject(
	ctx context.Context,
	childID, subject string,
	stages []string,
	primaryRank, target, need int,
	rng *rand.Rand,
	chosen map[string]bool,
	selected *[]Selected,
	addWarning func(string),
) error {
	if need <= 0 {
		return nil
	}

	fresh, err := s.repo.candidates(ctx, childID, subject, stages, true, defaultPoolLimit)
	if err != nil {
		return err
	}
	sortCandidates(fresh, rng, primaryRank, target)

	picked := 0
	for _, candidate := range fresh {
		if picked >= need {
			break
		}
		if chosen[candidate.ID] {
			continue
		}
		*selected = append(*selected, toSelected(candidate, false))
		chosen[candidate.ID] = true
		picked++
		if primaryRank >= 0 && stageRanks[candidate.Stage] != primaryRank {
			addWarning("stage_fallback:" + subject)
		}
		if candidate.Difficulty != target {
			addWarning("difficulty_fallback:" + subject)
		}
	}

	if picked >= need || childID == "" {
		if picked < need {
			addWarning("subject_shortage:" + subject)
		}
		return nil
	}

	// 去重导致不足时，退化为允许重复题。
	all, err := s.repo.candidates(ctx, childID, subject, stages, false, defaultPoolLimit)
	if err != nil {
		return err
	}
	sortCandidates(all, rng, primaryRank, target)
	for _, candidate := range all {
		if picked >= need {
			break
		}
		if chosen[candidate.ID] {
			continue
		}
		*selected = append(*selected, toSelected(candidate, true))
		chosen[candidate.ID] = true
		picked++
		addWarning("repeat_question:" + subject)
	}
	if picked < need {
		addWarning("subject_shortage:" + subject)
	}
	return nil
}

func toSelected(c Candidate, repeated bool) Selected {
	return Selected{
		QuestionID: c.ID,
		Subject:    c.Subject,
		Stage:      c.Stage,
		Type:       c.Type,
		Difficulty: c.Difficulty,
		Repeated:   repeated,
	}
}

func sortCandidates(items []Candidate, rng *rand.Rand, primaryRank, target int) {
	rng.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := stageRanks[items[i].Stage], stageRanks[items[j].Stage]
		if primaryRank >= 0 {
			di, dj := abs(ri-primaryRank), abs(rj-primaryRank)
			if di != dj {
				return di < dj
			}
		}
		di, dj := abs(items[i].Difficulty-target), abs(items[j].Difficulty-target)
		if di != dj {
			return di < dj
		}
		return false
	})
}

func sortedKeys(quota map[string]int) []string {
	keys := make([]string, 0, len(quota))
	for key := range quota {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func targetDifficulty(stage, grade string, stats map[int]float64) int {
	best, bestDistance := 0, math.MaxFloat64
	for difficulty, rate := range stats {
		distance := math.Abs(rate - 0.7)
		if distance < bestDistance {
			best, bestDistance = difficulty, distance
		}
	}
	if best > 0 && bestDistance <= 0.25 {
		return best
	}

	base := map[string]int{"preschool": 1, "low": 2, "mid": 3, "high": 3}[stage]
	if len(grade) == 2 && grade[0] == 'g' {
		switch grade[1] {
		case '1', '2', '3':
			base--
		case '7', '8', '9':
			base++
		}
	}
	return clamp(base, 1, 5)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
