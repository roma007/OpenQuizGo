package question

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roma007/OpenQuizGo/internal/admin"
)

var (
	gradePattern = regexp.MustCompile(`^g[1-9]$`)
	coreSubjects = map[string]bool{"math": true, "chinese": true, "english": true}
	litSubjects  = map[string]bool{"science": true, "logic": true, "safety": true}
)

// APIError 是带 HTTP 状态与错误码的业务错误。
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Message }

func newAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

var (
	errNotFound = newAPIError(404, "NOT_FOUND", "题目不存在")
	errConflict = newAPIError(409, "QUESTION_STATE_CONFLICT", "当前状态不允许该操作")
)

var transitions = map[string]map[string]string{
	"submit_review": {"draft": "review"},
	"approve":       {"review": "trial"},
	"reject":        {"review": "draft"},
	"publish":       {"trial": "online"},
	"offline":       {"online": "offline"},
	"resubmit":      {"offline": "review"},
}

// Service 承载题库治理逻辑。
type Service struct {
	repo   *repository
	logger *slog.Logger
}

// New 构造题库服务。
func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{repo: newRepository(pool), logger: logger}
}

// Create 创建题目（草稿）。
func (s *Service) Create(ctx context.Context, in Input, principal admin.Principal, ip string) (Question, error) {
	tags, err := s.repo.tags(ctx)
	if err != nil {
		return Question{}, err
	}
	topicIDs, err := s.repo.topicIDs(ctx)
	if err != nil {
		return Question{}, err
	}
	if problems := validate(&in, tags, topicIDs); len(problems) > 0 {
		return Question{}, newAPIError(422, "VALIDATION_ERROR", problems[0].Field+": "+problems[0].Message)
	}
	q, err := s.repo.create(ctx, in, principal.ID)
	if err != nil {
		return Question{}, err
	}
	s.audit(ctx, principal, "question.create", q.ID, q, ip)
	return q, nil
}

// Update 更新题目（仅 draft/offline），版本号 +1。
func (s *Service) Update(ctx context.Context, id string, in Input, principal admin.Principal, ip string) (Question, error) {
	current, err := s.repo.get(ctx, id)
	if isNoRows(err) {
		return Question{}, errNotFound
	}
	if err != nil {
		return Question{}, err
	}
	if current.Status != "draft" && current.Status != "offline" {
		return Question{}, newAPIError(409, "QUESTION_NOT_EDITABLE", "仅草稿或已下线题目可编辑")
	}
	tags, err := s.repo.tags(ctx)
	if err != nil {
		return Question{}, err
	}
	topicIDs, err := s.repo.topicIDs(ctx)
	if err != nil {
		return Question{}, err
	}
	if problems := validate(&in, tags, topicIDs); len(problems) > 0 {
		return Question{}, newAPIError(422, "VALIDATION_ERROR", problems[0].Field+": "+problems[0].Message)
	}
	q, err := s.repo.update(ctx, id, in, principal.ID)
	if isNoRows(err) {
		return Question{}, errConflict
	}
	if err != nil {
		return Question{}, err
	}
	s.audit(ctx, principal, "question.update", q.ID, q, ip)
	return q, nil
}

// Get 返回题目详情。
func (s *Service) Get(ctx context.Context, id string) (Question, error) {
	q, err := s.repo.get(ctx, id)
	if isNoRows(err) {
		return Question{}, errNotFound
	}
	return q, err
}

// List 四维检索。
const (
	calibrationUp   = 0.85
	calibrationDown = 0.40
)

// CalibrationPreview 返回难度校准建议。
func (s *Service) CalibrationPreview(ctx context.Context, minAnswers, limit int) (CalibrationResult, error) {
	if minAnswers <= 0 {
		minAnswers = 20
	}
	if minAnswers < 5 {
		minAnswers = 5
	}
	if minAnswers > 1000 {
		minAnswers = 1000
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.repo.calibrationRows(ctx, minAnswers, limit)
	if err != nil {
		return CalibrationResult{}, err
	}
	items := make([]CalibrationItem, 0, len(rows))
	for _, row := range rows {
		accuracy := 0.0
		if row.Answered > 0 {
			accuracy = float64(row.Correct) / float64(row.Answered)
		}
		suggested, reason := suggestDifficulty(row.Difficulty, row.Answered, row.Correct)
		items = append(items, CalibrationItem{
			QuestionID:          row.ID,
			Stem:                row.Stem,
			Subject:             row.Subject,
			Stage:               row.Stage,
			Difficulty:          row.Difficulty,
			Answered:            row.Answered,
			Correct:             row.Correct,
			Accuracy:            accuracy,
			SuggestedDifficulty: suggested,
			Reason:              reason,
		})
	}
	return CalibrationResult{
		Items: items,
		Thresholds: CalibrationThresholds{
			MinAnswers: minAnswers,
			Up:         calibrationUp,
			Down:       calibrationDown,
		},
	}, nil
}

// suggestDifficulty 根据正确率给出难度建议。
func suggestDifficulty(difficulty, answered, correct int) (int, string) {
	if answered <= 0 {
		return difficulty, "样本不足，保持难度"
	}
	accuracy := float64(correct) / float64(answered)
	if accuracy >= calibrationUp && difficulty < 5 {
		return difficulty + 1, "正确率偏高，建议提高难度"
	}
	if accuracy <= calibrationDown && difficulty > 1 {
		return difficulty - 1, "正确率偏低，建议降低难度"
	}
	return difficulty, "正确率在目标区间，保持难度"
}

// ApplyCalibration 应用难度调整。
func (s *Service) ApplyCalibration(ctx context.Context, items []CalibrationApplyItem,
	principal admin.Principal, ip string) (int, error) {
	if len(items) == 0 || len(items) > 500 {
		return 0, newAPIError(422, "VALIDATION_ERROR", "校准条目需在 1–500 之间")
	}
	seen := make(map[string]bool, len(items))
	clean := make([]CalibrationApplyItem, 0, len(items))
	for _, item := range items {
		if item.QuestionID == "" || item.Difficulty < 1 || item.Difficulty > 5 {
			return 0, newAPIError(422, "VALIDATION_ERROR", "题目或难度不合法")
		}
		if seen[item.QuestionID] {
			continue
		}
		seen[item.QuestionID] = true
		clean = append(clean, item)
	}
	return s.repo.applyCalibration(ctx, clean, principal.ID, ip)
}

// CalibrationHistory 返回难度校准历史，limit 默认 20、最大 200。
func (s *Service) CalibrationHistory(ctx context.Context, questionID string, limit int) ([]CalibrationHistoryItem, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	return s.repo.calibrationHistory(ctx, questionID, limit)
}

// Coverage 返回题库覆盖率聚合。
func (s *Service) Coverage(ctx context.Context, threshold int) (CoverageResult, error) {
	if threshold < 1 || threshold > 10000 {
		threshold = coverageMinOnline
	}
	cells, total, err := s.repo.Coverage(ctx)
	if err != nil {
		return CoverageResult{}, err
	}
	online := make(map[string]int)
	for _, cell := range cells {
		if cell.Status == "online" {
			online[cell.Subject+"|"+cell.Stage] += cell.Count
		}
	}
	warnings := make([]string, 0)
	for key, count := range online {
		if count >= threshold {
			continue
		}
		parts := strings.SplitN(key, "|", 2)
		warnings = append(warnings, fmt.Sprintf("%s/%s 在线题量 %d，低于建议 %d",
			subjectName(parts[0]), stageName(parts[1]), count, threshold))
	}
	sort.Strings(warnings)
	return CoverageResult{Cells: cells, Total: total, Warnings: warnings}, nil
}

const coverageMinOnline = 50

func subjectName(code string) string {
	switch code {
	case "math":
		return "数学"
	case "chinese":
		return "语文"
	case "english":
		return "英语"
	case "science":
		return "科学"
	case "logic":
		return "逻辑"
	case "safety":
		return "安全"
	case "fun":
		return "趣味"
	default:
		return code
	}
}

func stageName(code string) string {
	switch code {
	case "preschool":
		return "学龄前"
	case "low":
		return "小学低年级"
	case "mid":
		return "小学中年级"
	case "high":
		return "小学高年级"
	default:
		return code
	}
}

func (s *Service) List(ctx context.Context, f Filter) (ListResult, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.repo.list(ctx, f)
}

// Delete 删除草稿题目。
func (s *Service) Delete(ctx context.Context, id string, principal admin.Principal, ip string) error {
	ok, err := s.repo.deleteDraft(ctx, id)
	if err != nil {
		return err
	}
	if ok {
		s.audit(ctx, principal, "question.delete", id, map[string]string{"id": id}, ip)
		return nil
	}
	if _, err := s.repo.get(ctx, id); isNoRows(err) {
		return errNotFound
	}
	return errConflict
}

// Transition 执行状态机流转。
func (s *Service) Transition(ctx context.Context, id, action string, principal admin.Principal, ip string) (Question, error) {
	targets, ok := transitions[action]
	if !ok {
		return Question{}, newAPIError(422, "VALIDATION_ERROR", "未知操作："+action)
	}
	current, err := s.repo.get(ctx, id)
	if isNoRows(err) {
		return Question{}, errNotFound
	}
	if err != nil {
		return Question{}, err
	}
	to, ok := targets[current.Status]
	if !ok {
		return Question{}, newAPIError(409, "QUESTION_STATE_CONFLICT",
			"当前状态 "+current.Status+" 不支持操作 "+action)
	}
	updated, err := s.repo.transition(ctx, id, current.Status, to, principal.ID)
	if isNoRows(err) {
		return Question{}, errConflict
	}
	if err != nil {
		return Question{}, err
	}
	s.audit(ctx, principal, "question."+action, id, updated, ip)
	return updated, nil
}

// Import 批量导入草稿题目；任一行校验失败则整批不落库。
func (s *Service) Import(ctx context.Context, inputs []Input, principal admin.Principal, ip string) (ImportResult, error) {
	if len(inputs) == 0 {
		return ImportResult{}, newAPIError(422, "VALIDATION_ERROR", "导入内容为空")
	}
	if len(inputs) > 5000 {
		return ImportResult{}, newAPIError(422, "VALIDATION_ERROR", "单次最多导入 5000 条")
	}
	tags, err := s.repo.tags(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	topicIDs, err := s.repo.topicIDs(ctx)
	if err != nil {
		return ImportResult{}, err
	}

	result := ImportResult{Imported: 0}
	for i := range inputs {
		problems := validate(&inputs[i], tags, topicIDs)
		for j := range problems {
			problems[j].Row = i + 1
			result.Errors = append(result.Errors, problems[j])
		}
	}
	if len(result.Errors) > 0 {
		return result, nil
	}

	if err := s.repo.insertMany(ctx, inputs, principal.ID); err != nil {
		return ImportResult{}, err
	}
	result.Imported = len(inputs)
	s.audit(ctx, principal, "question.import", "", map[string]int{"imported": len(inputs)}, ip)
	return result, nil
}

// Audit 返回题目审计日志。
func (s *Service) Audit(ctx context.Context, filter AuditFilter) ([]AuditEntry, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	return s.repo.listAudit(ctx, filter)
}

// AuditExport 返回用于导出的审计记录，最多 5000 条。
func (s *Service) AuditExport(ctx context.Context, filter AuditFilter) ([]AuditEntry, error) {
	if filter.Limit <= 0 || filter.Limit > 5000 {
		filter.Limit = 5000
	}
	return s.repo.listAudit(ctx, filter)
}

func (s *Service) audit(ctx context.Context, principal admin.Principal, action, targetID string, after any, ip string) {
	if err := s.repo.audit(ctx, principal.ID, action, targetID, after, ip); err != nil {
		s.logger.Error("write audit log", "error", err, "action", action, "target_id", targetID)
	}
}

/* ------------------------------ 校验 ------------------------------ */

type optionItem struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Image string `json:"image"`
}

func validate(in *Input, tags tagSets, topicIDs map[string]bool) []ImportError {
	problems := make([]ImportError, 0)
	add := func(field, message string) {
		problems = append(problems, ImportError{Field: field, Message: message})
	}

	in.Subject = strings.TrimSpace(in.Subject)
	in.Category = strings.TrimSpace(in.Category)
	in.Stage = strings.TrimSpace(in.Stage)
	in.Type = strings.TrimSpace(in.Type)
	in.GradeTag = strings.TrimSpace(in.GradeTag)
	in.Stem = strings.TrimSpace(in.Stem)

	if !tags.subjects[in.Subject] {
		add("subject", "学科不存在或已禁用")
	}
	if !tags.categories[in.Category] {
		add("category", "类目不存在或已禁用")
	}
	if !tags.stages[in.Stage] {
		add("stage", "学段不存在或已禁用")
	}
	if !tags.types[in.Type] {
		add("type", "题型不存在或已禁用")
	}
	if in.Difficulty < 1 || in.Difficulty > 5 {
		add("difficulty", "难度需在 1–5 之间")
	}
	if n := utf8.RuneCountInString(in.Stem); n < 1 || n > 2000 {
		add("stem", "题干长度需在 1–2000 字之间")
	}
	if in.GradeTag != "" && !gradePattern.MatchString(in.GradeTag) {
		add("grade_tag", "年级标签需为 g1–g9")
	}
	if len(in.Explain) > 2000 {
		add("explain", "解析过长")
	}

	// 学科与类目一致性
	switch in.Category {
	case "fun":
		if in.Subject != "fun" {
			add("subject", "趣味类目必须使用 fun 学科")
		}
	case "core":
		if !coreSubjects[in.Subject] {
			add("category", "核心学科只能使用 math/chinese/english")
		}
	case "literacy":
		if !litSubjects[in.Subject] {
			add("category", "素养学科只能使用 science/logic/safety")
		}
	case "topic":
		if in.Subject == "fun" {
			add("category", "专题内容不能使用 fun 学科")
		}
	}
	if in.Subject == "fun" && in.Category != "fun" {
		add("category", "fun 学科必须归入趣味类目")
	}

	// 专题归属
	if in.TopicID != nil && *in.TopicID != "" {
		if !topicIDs[*in.TopicID] {
			add("topic_id", "专题题包不存在")
		}
	} else if in.Category == "topic" {
		add("topic_id", "专题内容必须指定 topic_id")
	}

	// JSON 合法性
	if len(in.Answer) == 0 || !json.Valid(in.Answer) || string(in.Answer) == "null" {
		add("answer", "标准答案必填且必须是合法 JSON")
	}
	if len(in.Options) > 0 && !json.Valid(in.Options) {
		add("options", "options 不是合法 JSON")
	}
	if len(in.AnswerAlias) > 0 && !json.Valid(in.AnswerAlias) {
		add("answer_alias", "answer_alias 不是合法 JSON")
	}
	if len(in.StemMedia) > 0 && !json.Valid(in.StemMedia) {
		add("stem_media", "stem_media 不是合法 JSON")
	}

	// 题型针对性校验
	if problems == nil || len(problems) == 0 {
		problems = append(problems, validateByType(in)...)
	}

	// 趣味题：近义答案必填
	if in.Category == "fun" {
		var aliases []string
		if len(in.AnswerAlias) == 0 || json.Unmarshal(in.AnswerAlias, &aliases) != nil || len(aliases) == 0 {
			add("answer_alias", "趣味题必须提供至少 1 条近义答案")
		}
	}

	// 单题限时
	if in.DurationLimit == nil {
		limit := 30
		if in.Category == "fun" {
			limit = 15
		}
		in.DurationLimit = &limit
	}
	if in.Category == "fun" {
		if *in.DurationLimit < 10 || *in.DurationLimit > 20 {
			add("duration_limit", "趣味题限时需在 10–20 秒")
		}
	} else if *in.DurationLimit < 5 || *in.DurationLimit > 30 {
		add("duration_limit", "学科题限时需在 5–30 秒")
	}

	if in.SchemaVer == 0 {
		in.SchemaVer = 1
	}
	if in.SchemaVer < 1 {
		add("schema_ver", "schema_ver 必须为正整数")
	}
	return problems
}

func validateByType(in *Input) []ImportError {
	problems := make([]ImportError, 0)
	add := func(field, message string) {
		problems = append(problems, ImportError{Field: field, Message: message})
	}

	switch in.Type {
	case "choice", "image":
		if len(in.Options) == 0 {
			add("options", "选择题/看图题必须提供选项")
			return problems
		}
		var options []optionItem
		if err := json.Unmarshal(in.Options, &options); err != nil || len(options) < 2 || len(options) > 8 {
			add("options", "选项需为 2–8 个 {id,text} 对象")
			return problems
		}
		ids := map[string]bool{}
		for _, opt := range options {
			if opt.ID == "" || (opt.Text == "" && opt.Image == "") || ids[opt.ID] {
				add("options", "选项 id 不能为空、id 不能重复，且 text/image 至少一个")
				return problems
			}
			ids[opt.ID] = true
		}
		var answers []string
		if err := json.Unmarshal(in.Answer, &answers); err != nil || len(answers) == 0 {
			add("answer", "答案需为选项 id 数组")
			return problems
		}
		for _, id := range answers {
			if !ids[id] {
				add("answer", "答案包含不存在的选项 id："+id)
				return problems
			}
		}
	case "voice", "fill":
		if !isStringOrStringArray(in.Answer) {
			add("answer", "答案需为字符串或字符串数组")
		}
	case "sort":
		if len(in.Options) == 0 {
			add("options", "排序题必须提供选项")
		}
		if !isStringOrStringArray(in.Answer) {
			add("answer", "答案需为字符串或字符串数组")
		}
	case "match":
		var pairs map[string]any
		if json.Unmarshal(in.Answer, &pairs) != nil || len(pairs) == 0 {
			add("answer", "连线题答案需为对象")
		}
	}
	return problems
}

func isStringOrStringArray(raw json.RawMessage) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil && single != "" {
		return true
	}
	var many []string
	return json.Unmarshal(raw, &many) == nil && len(many) > 0
}
