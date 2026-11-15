package quiz

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/platform/metrics"
	"github.com/roma007/OpenQuizGo/internal/rule"
	"github.com/roma007/OpenQuizGo/internal/watch"
)

const (
	sessionCreateLimit  = 3
	sessionCreateWindow = 10 * time.Minute
	answerLimit         = 60
	answerWindow        = time.Minute
	suspiciousMS        = 300
)

// CreateSession 为设备上的孩子创建答题会话。
func (s *Service) CreateSession(ctx context.Context, principal identity.DevicePrincipal) (CreateSessionResponse, error) {
	if !s.allow(ctx, "rl:quiz:create:"+principal.DeviceID, sessionCreateLimit, sessionCreateWindow) {
		return CreateSessionResponse{}, newError(429, "QUIZ_SESSION_LIMIT", "发起答题过于频繁，请稍后再试")
	}
	if existing, err := s.repo.activeSessionForDevice(ctx, principal.DeviceID); err == nil {
		conflict := newError(409, "QUIZ_SESSION_ACTIVE", "已有进行中的答题会话")
		conflict.SessionID = existing
		return CreateSessionResponse{}, conflict
	}

	ruleSnapshot, err := s.loadRuleForDevice(ctx, principal)
	if err != nil {
		return CreateSessionResponse{}, err
	}

	result, err := s.Select(ctx, SelectRequest{
		ChildID:       principal.ChildID,
		Rule:          ruleSnapshot.Rule,
		Seed:          time.Now().UnixNano(),
		Random:        ruleSnapshot.Random,
		Subjects:      ruleSnapshot.Subjects,
		QuestionCount: ruleSnapshot.QuestionCount,
	})
	if err != nil {
		return CreateSessionResponse{}, err
	}
	if len(result.Selected) == 0 {
		return CreateSessionResponse{}, newError(409, "INSUFFICIENT_POOL", "题库不足，无法组卷")
	}

	ids := make([]string, 0, len(result.Selected))
	for _, item := range result.Selected {
		ids = append(ids, item.QuestionID)
	}
	details, err := s.repo.questionDetails(ctx, ids)
	if err != nil {
		return CreateSessionResponse{}, err
	}

	now := time.Now()
	expiresAt := now.Add(sessionTTL)
	items := make([]itemRecord, 0, len(result.Selected))
	questions := make([]SessionQuestion, 0, len(result.Selected))
	for position, selected := range result.Selected {
		detail, ok := details[selected.QuestionID]
		if !ok {
			return CreateSessionResponse{}, newError(409, "QUESTION_OFFLINE", "题目已下线，请重新发起")
		}
		snapshot := itemSnapshot{
			Subject:       detail.Subject,
			Type:          detail.Type,
			Answer:        detail.Answer,
			Alias:         detail.Alias,
			Explain:       detail.Explain,
			Stem:          detail.Stem,
			StemMedia:     detail.StemMedia,
			Options:       detail.Options,
			DurationLimit: detail.DurationLimit,
		}
		encoded, err := json.Marshal(snapshot)
		if err != nil {
			return CreateSessionResponse{}, err
		}
		token := randomToken()
		items = append(items, itemRecord{
			QuestionID:      detail.ID,
			QuestionVersion: detail.Version,
			Position:        position,
			Token:           token,
			Snapshot:        encoded,
		})
		questions = append(questions, SessionQuestion{
			ItemID:        "",
			QuestionID:    detail.ID,
			Position:      position,
			Type:          detail.Type,
			Stem:          detail.Stem,
			StemMedia:     detail.StemMedia,
			Options:       detail.Options,
			DurationLimit: detail.DurationLimit,
			Token:         token,
		})
	}

	// 以实际选中的题为准再兜底一次：题池受限时（尤其 1–2 题或专题题包）
	// 门槛不得超过本次实际可拿到的最高分，否则孩子永远答不过（082）。
	passThreshold := ruleSnapshot.Rule.PassThreshold
	if result.MaxScore > 0 && passThreshold > result.MaxScore {
		passThreshold = result.MaxScore
	}
	// 通过标准按答对题数：不超过实际下发的题量，至少 1 题。
	passQuestions := ruleSnapshot.PassQuestions
	if passQuestions <= 0 {
		passQuestions = ruleSnapshot.Rule.PassQuestions
	}
	if passQuestions <= 0 {
		passQuestions = 2
	}
	if passQuestions > len(result.Selected) {
		passQuestions = len(result.Selected)
	}
	if passQuestions < 1 {
		passQuestions = 1
	}
	sessionID, err := s.repo.createSession(ctx, principal.ChildID, principal.DeviceID,
		ruleSnapshot, passThreshold, passQuestions, expiresAt, items)
	if err != nil {
		return CreateSessionResponse{}, err
	}
	metrics.IncQuizSession()
	return CreateSessionResponse{
		SessionID:     sessionID,
		ExpiresAt:     expiresAt,
		PassThreshold: passThreshold,
		PassQuestions: passQuestions,
		RuleVersion:   ruleSnapshot.Version,
		Questions:     questions,
	}, nil
}

// SubmitAnswer 校验一次性题令牌并服务端判分。
func (s *Service) SubmitAnswer(ctx context.Context, principal identity.DevicePrincipal,
	sessionID string, submission AnswerSubmission) (AnswerResult, error) {
	if !s.allow(ctx, "rl:quiz:answer:"+principal.DeviceID, answerLimit, answerWindow) {
		return AnswerResult{}, newError(429, "RATE_LIMITED", "答题过于频繁，请稍后再试")
	}
	session, err := s.loadSession(ctx, principal, sessionID)
	if err != nil {
		return AnswerResult{}, err
	}
	if session.State != "active" {
		return AnswerResult{}, newError(409, "QUIZ_SESSION_CLOSED", "答题会话已结束")
	}
	if time.Now().After(session.ExpiresAt) {
		_ = s.repo.setSessionState(ctx, session.ID, "expired")
		return AnswerResult{}, newError(410, "QUIZ_SESSION_EXPIRED", "答题会话已超时")
	}

	item, err := s.repo.itemByToken(ctx, session.ID, submission.Token)
	if err != nil {
		if isNoRows(err) {
			return AnswerResult{}, newError(404, "QUIZ_ITEM_NOT_FOUND", "题目不存在")
		}
		return AnswerResult{}, err
	}
	if item.AnsweredAt != nil {
		return AnswerResult{}, newError(409, "QUIZ_TOKEN_USED", "该题已作答，令牌已失效")
	}

	var snapshot itemSnapshot
	if err := json.Unmarshal(item.Snapshot, &snapshot); err != nil {
		return AnswerResult{}, err
	}
	correct, err := grade(snapshot, submission.Answer)
	if err != nil {
		return AnswerResult{}, newError(422, "INVALID_ANSWER", "答案格式不合法")
	}

	durationMS := int(time.Since(item.IssuedAt).Milliseconds())
	if durationMS < 0 {
		durationMS = 0
	}
	marked, err := s.repo.markAnswered(ctx, item.ID, submission.Answer, correct, durationMS)
	if err != nil {
		return AnswerResult{}, err
	}
	if !marked {
		return AnswerResult{}, newError(409, "QUIZ_TOKEN_USED", "该题已作答，令牌已失效")
	}
	metrics.IncQuizAnswer(correct)

	var rule ruleSnapshot
	_ = json.Unmarshal(session.RuleSnapshot, &rule)
	weight := rule.Rule.SubjectWeight[snapshot.Subject]
	delta := 0.0
	if correct {
		delta = weight
	}
	score, _, err := s.repo.addScore(ctx, session.ID, delta, boolToInt(correct))
	if err != nil {
		return AnswerResult{}, err
	}
	if err := s.repo.insertAnswerLog(ctx, session.ChildID, item.QuestionID, session.ID,
		principal.DeviceID, submission.Answer, correct, durationMS); err != nil {
		s.logger.Error("insert answer log", "error", err, "session_id", session.ID)
	}

	answered, err := s.repo.answeredCount(ctx, session.ID)
	if err != nil {
		return AnswerResult{}, err
	}
	return AnswerResult{
		Correct:       correct,
		CorrectAnswer: snapshot.Answer,
		Explain:       snapshot.Explain,
		DurationMS:    durationMS,
		Suspicious:    durationMS < suspiciousMS,
		Score:         score,
		Threshold:     session.PassThreshold,
		PassQuestions: session.PassQuestions,
		Answered:      answered,
		Total:         session.QuestionCount,
		Remaining:     session.QuestionCount - answered,
	}, nil
}

// FinishSession 结算会话：答对题数达到要求即通过（083）。
func (s *Service) FinishSession(ctx context.Context, principal identity.DevicePrincipal, sessionID string) (SessionSummary, error) {
	session, err := s.loadSession(ctx, principal, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}
	if session.State != "active" {
		summary, err := s.summary(ctx, session, false)
		if err == nil && session.State == "passed" {
			s.attachGrant(ctx, session, &summary)
		}
		return summary, err
	}
	if time.Now().After(session.ExpiresAt) {
		_ = s.repo.setSessionState(ctx, session.ID, "expired")
		session.State = "expired"
		return s.summary(ctx, session, false)
	}
	state := "failed"
	if sessionPassed(session.CorrectCount, session.PassQuestions) {
		state = "passed"
	}
	if err := s.repo.setSessionState(ctx, session.ID, state); err != nil {
		return SessionSummary{}, err
	}
	session.State = state
	now := time.Now()
	session.FinishedAt = &now
	summary, err := s.summary(ctx, session, false)
	if err != nil {
		return SessionSummary{}, err
	}
	if state == "passed" {
		s.issueGrant(ctx, session, &summary)
	}
	return summary, nil
}

// sessionPassed 是按答对题数判定的唯一入口，便于测试与后续调整。
func sessionPassed(correctCount, passQuestions int) bool {
	if passQuestions < 1 {
		passQuestions = 1
	}
	return correctCount >= passQuestions
}

func (s *Service) issueGrant(ctx context.Context, session sessionRecord, summary *SessionSummary) {
	if s.watchSvc == nil {
		return
	}
	var snapshot ruleSnapshot
	_ = json.Unmarshal(session.RuleSnapshot, &snapshot)
	grant, err := s.watchSvc.Issue(ctx, watch.IssueRequest{
		SessionID: session.ID,
		ChildID:   session.ChildID,
		DeviceID:  session.DeviceID,
		Rule: watch.RuleView{
			UnlockMinutes: snapshot.Rule.UnlockMinutes,
			DailyLimit:    snapshot.Rule.DailyLimit,
			Timezone:      snapshot.Rule.Timezone,
		},
	})
	if err != nil {
		summary.GrantError = err.Error()
		s.logger.Error("issue watch grant", "error", err, "session_id", session.ID)
		return
	}
	summary.Grant = &grant
}

func (s *Service) attachGrant(ctx context.Context, session sessionRecord, summary *SessionSummary) {
	if s.watchSvc == nil {
		return
	}
	grant, err := s.watchSvc.GrantForSession(ctx, session.ID)
	if err != nil {
		summary.GrantError = err.Error()
		return
	}
	summary.Grant = &grant
}

// GetSession 返回会话状态（用于断线重连）。
func (s *Service) GetSession(ctx context.Context, principal identity.DevicePrincipal, sessionID string) (SessionSummary, error) {
	session, err := s.loadSession(ctx, principal, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}
	if session.State == "active" && time.Now().After(session.ExpiresAt) {
		_ = s.repo.setSessionState(ctx, session.ID, "expired")
		session.State = "expired"
	}
	return s.summary(ctx, session, true)
}

func (s *Service) loadSession(ctx context.Context, principal identity.DevicePrincipal, sessionID string) (sessionRecord, error) {
	session, err := s.repo.sessionByID(ctx, sessionID)
	if err != nil {
		if isNoRows(err) {
			return sessionRecord{}, newError(404, "QUIZ_SESSION_NOT_FOUND", "答题会话不存在")
		}
		return sessionRecord{}, err
	}
	if session.ChildID != principal.ChildID || session.DeviceID != principal.DeviceID {
		return sessionRecord{}, newError(404, "QUIZ_SESSION_NOT_FOUND", "答题会话不存在")
	}
	return session, nil
}

func (s *Service) summary(ctx context.Context, session sessionRecord, withQuestions bool) (SessionSummary, error) {
	answered, err := s.repo.answeredCount(ctx, session.ID)
	if err != nil {
		return SessionSummary{}, err
	}
	summary := SessionSummary{
		SessionID:     session.ID,
		State:         session.State,
		Score:         session.Score,
		Threshold:     session.PassThreshold,
		PassQuestions: session.PassQuestions,
		CorrectCount:  session.CorrectCount,
		AnsweredCount: answered,
		QuestionCount: session.QuestionCount,
		StartedAt:     session.StartedAt,
		ExpiresAt:     session.ExpiresAt,
		FinishedAt:    session.FinishedAt,
	}
	if !withQuestions {
		return summary, nil
	}
	items, err := s.repo.itemsBySession(ctx, session.ID)
	if err != nil {
		return SessionSummary{}, err
	}
	questions := make([]SessionQuestion, 0, len(items))
	for _, item := range items {
		var snapshot itemSnapshot
		_ = json.Unmarshal(item.Snapshot, &snapshot)
		questions = append(questions, SessionQuestion{
			ItemID:        item.ID,
			QuestionID:    item.QuestionID,
			Position:      item.Position,
			Type:          snapshot.Type,
			Stem:          snapshot.Stem,
			StemMedia:     snapshot.StemMedia,
			Options:       snapshot.Options,
			DurationLimit: snapshot.DurationLimit,
			Token:         item.Token,
			Answered:      item.AnsweredAt != nil,
		})
	}
	summary.Questions = questions
	return summary, nil
}

func (s *Service) allow(ctx context.Context, key string, limit int, window time.Duration) bool {
	if s.redis == nil {
		return true
	}
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		s.logger.Error("rate limit", "error", err, "key", key)
		return true
	}
	if count == 1 {
		_ = s.redis.Expire(ctx, key, window).Err()
	}
	return count <= int64(limit)
}

// loadRuleForDevice 按设备类型取规则：081 起的电视用按电视配置合成，
// 旧版激活码设备仍按孩子规则，保持兼容。
func (s *Service) loadRuleForDevice(ctx context.Context, principal identity.DevicePrincipal) (ruleSnapshot, error) {
	if principal.Kind != "tv" || s.ruleSvc == nil {
		return s.loadRule(ctx, principal.ChildID)
	}
	plan, err := s.ruleSvc.CanStartDevice(ctx, principal)
	if err != nil {
		var ruleErr *rule.Error
		if errors.As(err, &ruleErr) {
			return ruleSnapshot{}, newError(ruleErr.Status, ruleErr.Code, ruleErr.Message)
		}
		return ruleSnapshot{}, err
	}
	return ruleSnapshot{
		Version:       plan.Version,
		Rule:          plan.Rule,
		Random:        plan.Random,
		Subjects:      plan.Subjects,
		QuestionCount: plan.QuestionCount,
		PassQuestions: plan.PassQuestions,
	}, nil
}

func (s *Service) loadRule(ctx context.Context, childID string) (ruleSnapshot, error) {
	if s.ruleSvc != nil {
		r, version, err := s.ruleSvc.CanStart(ctx, childID)
		if err == nil {
			return ruleSnapshot{Version: version, Rule: r}, nil
		}
		var ruleErr *rule.Error
		if errors.As(err, &ruleErr) {
			if ruleErr.Code == "RULE_NOT_FOUND" {
				return defaultRule(), nil
			}
			return ruleSnapshot{}, newError(ruleErr.Status, ruleErr.Code, ruleErr.Message)
		}
		return ruleSnapshot{}, err
	}
	snap, err := s.repo.ruleForChild(ctx, childID)
	if err != nil {
		if isNoRows(err) {
			return defaultRule(), nil
		}
		return ruleSnapshot{}, err
	}
	return snap, nil
}

func defaultRule() ruleSnapshot {
	return ruleSnapshot{
		Version: 0,
		Rule: Rule{
			UnlockMinutes: 15,
			PassThreshold: 3,
			PassQuestions: 2,
			SubjectWeight: map[string]float64{
				"math": 1.5, "chinese": 1.5, "english": 1.5,
				"science": 1, "logic": 1, "safety": 1, "fun": 0.5,
			},
			FunRatioCap: 2,
			FunEnabled:  true,
		},
	}
}

func randomToken() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
