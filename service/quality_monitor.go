package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
)

var ErrQualityMonitorBusy = errors.New("a quality monitor run is already active")

type QualityMonitorGroupOption struct {
	Name   string   `json:"name"`
	Models []string `json:"models"`
}
type QualityMonitorOptions struct {
	Models []string                    `json:"models"`
	Groups []QualityMonitorGroupOption `json:"groups"`
}

func QualityMonitorModels() []string { return []string{"gpt-6-astra", "gpt-6.1-sol"} }

// Probes use the existing Responses adapters without translating the request to
// a different upstream protocol. Other capabilities remain valid groups, but
// unsupported group/model combinations are persisted as skipped results.
func QualityMonitorChannelSupported(channelType int) bool {
	apiType, _ := common.ChannelType2APIType(channelType)
	// Codex deliberately drops max_output_tokens and therefore cannot honor the
	// plan's frozen output budget. Do not submit a probe through that adapter.
	return apiType == constant.APITypeOpenAI
}

func GetQualityMonitorOptions(allowedGroups map[string]string) (QualityMonitorOptions, error) {
	result := QualityMonitorOptions{Models: QualityMonitorModels(), Groups: make([]QualityMonitorGroupOption, 0)}
	var abilities []model.AbilityWithChannel
	err := model.DB.Table("abilities").Select("abilities.*, channels.type as channel_type").Joins("join channels on channels.id = abilities.channel_id").Where("abilities.enabled = ? AND channels.status = ?", true, common.ChannelStatusEnabled).Scan(&abilities).Error
	if err != nil {
		return result, err
	}
	groups := map[string]map[string]bool{}
	for _, a := range abilities {
		if a.Group == "auto" {
			continue
		}
		if allowedGroups != nil {
			if _, ok := allowedGroups[a.Group]; !ok {
				continue
			}
		}
		if groups[a.Group] == nil {
			groups[a.Group] = map[string]bool{}
		}
		if (a.Model == "gpt-6-astra" || a.Model == "gpt-6.1-sol") && QualityMonitorChannelSupported(a.ChannelType) {
			groups[a.Group][a.Model] = true
		}
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		models := make([]string, 0, 2)
		for _, m := range result.Models {
			if groups[name][m] {
				models = append(models, m)
			}
		}
		result.Groups = append(result.Groups, QualityMonitorGroupOption{Name: name, Models: models})
	}
	return result, nil
}

func ValidateQualityMonitorPlan(p *model.QualityMonitorPlan, existingGroups ...[]string) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || utf8.RuneCountInString(p.Name) > 100 {
		return errors.New("plan name must be 1–100 characters")
	}
	if len(p.Groups) < 1 || len(p.Groups) > 20 {
		return errors.New("select 1–20 groups")
	}
	if len(p.Models) < 1 || len(p.Models) > 2 {
		return errors.New("select 1–2 models")
	}
	if len(p.Questions) < 1 || len(p.Questions) > 5 {
		return errors.New("provide 1–5 questions")
	}
	if len(p.Groups)*len(p.Models)*len(p.Questions) > 100 {
		return errors.New("a run may contain at most 100 probes")
	}
	if p.IntervalMinutes < 1 || p.IntervalMinutes > 10080 {
		return errors.New("interval must be 1–10080 minutes")
	}
	if p.MaxOutputTokens < 128 || p.MaxOutputTokens > 8192 {
		return errors.New("output budget must be 128–8192 tokens")
	}
	if p.TimeoutSeconds < 10 || p.TimeoutSeconds > 180 {
		return errors.New("timeout must be 10–180 seconds")
	}
	switch p.ReasoningEffort {
	case "low", "medium", "high", "max":
	default:
		return errors.New("invalid reasoning effort")
	}
	seen := map[string]bool{}
	for _, m := range p.Models {
		if (m != "gpt-6-astra" && m != "gpt-6.1-sol") || seen[m] {
			return errors.New("invalid or duplicate model")
		}
		seen[m] = true
	}
	options, err := GetQualityMonitorOptions(nil)
	if err != nil {
		return err
	}
	active := map[string]bool{}
	for _, g := range options.Groups {
		active[g.Name] = true
	}
	if len(existingGroups) > 0 {
		for _, g := range existingGroups[0] {
			active[g] = true
		}
	}
	seen = map[string]bool{}
	for _, g := range p.Groups {
		if !active[g] || seen[g] {
			return errors.New("invalid or duplicate active group")
		}
		seen[g] = true
	}
	seen = map[string]bool{}
	for _, q := range p.Questions {
		if strings.TrimSpace(q.ID) == "" || len(q.ID) > 64 || seen[q.ID] {
			return errors.New("question ids must be unique and 1–64 bytes")
		}
		seen[q.ID] = true
		if strings.TrimSpace(q.Prompt) == "" || len(q.Prompt) > 8<<10 {
			return errors.New("question prompt must be 1–8192 bytes")
		}
		if len(q.ExpectedAnswer) > 8<<10 {
			return errors.New("reference answer exceeds 8192 bytes")
		}
		switch q.MatchType {
		case "manual":
		case "exact", "contains":
			if strings.TrimSpace(q.ExpectedAnswer) == "" {
				return errors.New("automatic grading requires a reference answer")
			}
		default:
			return errors.New("invalid answer match type")
		}
	}
	return nil
}

func EvaluateQualityMonitorAnswer(kind, answer, expected string) string {
	answer = strings.ToLower(strings.TrimSpace(answer))
	expected = strings.ToLower(strings.TrimSpace(expected))
	if kind == "manual" {
		return "ungraded"
	}
	if answer == "" || expected == "" {
		return "failed"
	}
	if kind == "exact" && answer == expected || kind == "contains" && strings.Contains(answer, expected) {
		return "passed"
	}
	return "failed"
}

type qualityMonitorTaskPayload struct {
	Plan *model.QualityMonitorPlan `json:"plan,omitempty"`
}
type QualityMonitorProbeResult struct {
	Answer              string
	AnswerTruncated     bool
	ActualResponseModel string
	EstimatedQuota      int
}

// The callback receives a frozen plan with the selected group and model as its
// sole Groups/Models entries; the other configuration fields retain the snapshot.
type QualityMonitorProbe func(context.Context, *model.Channel, model.QualityMonitorPlan, model.QualityMonitorQuestion) (QualityMonitorProbeResult, error)

func StartQualityMonitorRun(planID int) (*model.SystemTask, error) {
	plan, err := model.GetQualityMonitorPlan(planID)
	if err != nil {
		return nil, err
	}
	if active, err := model.GetActiveSystemTask(model.SystemTaskTypeQualityMonitor); err != nil {
		return nil, err
	} else if active != nil {
		return nil, ErrQualityMonitorBusy
	}
	task, err := model.CreateSystemTask(model.SystemTaskTypeQualityMonitor, qualityMonitorTaskPayload{Plan: plan}, nil)
	if err != nil {
		if active, lookupErr := model.GetActiveSystemTask(model.SystemTaskTypeQualityMonitor); lookupErr == nil && active != nil {
			return nil, ErrQualityMonitorBusy
		}
		return nil, err
	}
	notifySystemTaskRunner()
	return task, nil
}

func HasDueQualityMonitorPlans(now int64) bool {
	var count int64
	return model.DB.Model(&model.QualityMonitorPlan{}).Where("enabled = ? AND next_run_at <= ?", true, now).Count(&count).Error == nil && count > 0
}

func RunQualityMonitorTask(ctx context.Context, task *model.SystemTask, runnerID string, probe QualityMonitorProbe) (runErr error) {
	payload := qualityMonitorTaskPayload{}
	if err := task.DecodePayload(&payload); err != nil {
		return err
	}
	var plan model.QualityMonitorPlan
	if payload.Plan != nil {
		plan = *payload.Plan
	} else {
		err := model.DB.Where("enabled = ? AND next_run_at <= ?", true, common.GetTimestamp()).Order("next_run_at asc, id asc").First(&plan).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_, err = model.CleanupQualityMonitorResults(common.GetTimestamp())
			return err
		}
		if err != nil {
			return err
		}
	}
	now := common.GetTimestamp()
	calls := len(plan.Groups) * len(plan.Models) * len(plan.Questions)
	if calls < 1 || calls > 100 || plan.TimeoutSeconds < 10 || plan.TimeoutSeconds > 180 || plan.IntervalMinutes < 1 || plan.IntervalMinutes > 10080 {
		return errors.New("invalid stored quality monitor plan")
	}
	// Reserve the entire possible run before the first paid request. A crashed
	// worker is never resumed; the next run waits beyond this run's full budget.
	reservedNext := now + int64(calls*plan.TimeoutSeconds+plan.IntervalMinutes*60)
	if err := model.DB.Model(&model.QualityMonitorPlan{}).Where("id = ?", plan.ID).Updates(map[string]any{"last_run_at": now, "next_run_at": reservedNext}).Error; err != nil {
		return err
	}
	defer func() {
		if err := model.UpdateSystemTaskState(task.TaskID, runnerID, map[string]any{"plan_id": plan.ID, "completed": true}); err != nil {
			if runErr == nil {
				runErr = err
			}
			return
		}
		var current model.QualityMonitorPlan
		err := model.DB.First(&current, plan.ID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		if err != nil {
			if runErr == nil {
				runErr = err
			}
			return
		}
		next := int64(0)
		if current.Enabled {
			next = common.GetTimestamp() + int64(current.IntervalMinutes*60)
		}
		err = model.DB.Model(&model.QualityMonitorPlan{}).Where("id = ? AND last_run_at = ?", plan.ID, now).Updates(map[string]any{"next_run_at": next}).Error
		if runErr == nil {
			runErr = err
		}
	}()
	for _, group := range plan.Groups {
		for _, modelName := range plan.Models {
			for _, question := range plan.Questions {
				if err := ctx.Err(); err != nil {
					return err
				}
				// Fence every submission against the durable lease as well as heartbeat
				// cancellation. An old worker cannot submit the next probe after takeover.
				if err := model.UpdateSystemTaskState(task.TaskID, runnerID, map[string]any{"plan_id": plan.ID, "group": group, "model": modelName, "question_id": question.ID}); err != nil {
					return err
				}
				result := model.QualityMonitorResult{PlanID: plan.ID, PlanName: plan.Name, Group: group, Model: modelName, QuestionID: question.ID, Prompt: question.Prompt, ExpectedAnswer: question.ExpectedAnswer, ReasoningEffort: plan.ReasoningEffort, CreatedAt: common.GetTimestamp(), Status: "skipped"}
				channel, err := SelectQualityMonitorChannel(group, modelName)
				if err != nil {
					return err
				}
				if channel == nil {
					result.Error = "No compatible active channel for this group and model"
				} else {
					lease, acquired, err := model.AcquireChannelPoolLease(channel)
					if err != nil {
						return err
					}
					if !acquired {
						result.Error = "Channel capacity is currently unavailable"
					} else {
						result.ChannelID = channel.Id
						selected := plan
						selected.Groups = []string{group}
						selected.Models = []string{modelName}
						probeCtx, cancel := context.WithTimeout(ctx, time.Duration(plan.TimeoutSeconds)*time.Second)
						started := time.Now()
						response, probeErr := probe(probeCtx, channel, selected, question)
						result.DurationMS = time.Since(started).Milliseconds()
						if probeErr != nil {
							result.Status = "error"
							result.Error = "Upstream probe failed"
							var apiError *types.NewAPIError
							if errors.As(probeErr, &apiError) {
								switch apiError.GetErrorCode() {
								case types.ErrorCodeModelPriceError:
									result.Error = "Model pricing is not configured or invalid"
								case types.ErrorCodeDoRequestFailed:
									result.Error = "Unable to contact upstream"
								case types.ErrorCodeConvertRequestFailed:
									result.Error = "Provider request conversion failed"
								case types.ErrorCodeChannelModelMappedError:
									result.Error = "Channel model mapping is invalid"
								case types.ErrorCodeChannelParamOverrideInvalid:
									result.Error = "Channel request overrides are invalid"
								case types.ErrorCodeGetChannelFailed, types.ErrorCodeChannelNoAvailableKey:
									result.Error = "No available channel credential or provider account"
								case types.ErrorCodeBadResponseBody:
									result.Error = "Upstream returned an invalid or incomplete response"
								case types.ErrorCodeReadResponseBodyFailed:
									result.Error = "Unable to read upstream response"
								case types.ErrorCodeBadResponseStatusCode:
									if apiError.StatusCode >= 100 && apiError.StatusCode <= 599 {
										result.Error = fmt.Sprintf("Upstream HTTP %d", apiError.StatusCode)
									}
								}
							}
							if errors.Is(probeErr, context.DeadlineExceeded) || errors.Is(probeCtx.Err(), context.DeadlineExceeded) {
								result.Error = "Probe timed out"
							}
						} else {
							result.Answer = response.Answer
							result.AnswerTruncated = response.AnswerTruncated
							if len(result.Answer) > (64<<10)-1 {
								end := (64 << 10) - 1
								for end > 0 && !utf8.ValidString(result.Answer[:end]) {
									end--
								}
								result.Answer = result.Answer[:end]
								result.AnswerTruncated = true
							}
							result.ActualResponseModel = response.ActualResponseModel
							result.EstimatedQuota = response.EstimatedQuota
							result.Status = EvaluateQualityMonitorAnswer(question.MatchType, result.Answer, question.ExpectedAnswer)
							if result.AnswerTruncated && question.MatchType != "manual" {
								result.Status = "failed"
							}
						}
						cancel()
						lease.Release()
					}
				}
				if err := model.DB.Create(&result).Error; err != nil {
					return err
				}
			}
		}
	}
	_, err := model.CleanupQualityMonitorResults(common.GetTimestamp())
	return err
}

func SelectQualityMonitorChannel(group, modelName string) (*model.Channel, error) {
	var abilities []struct {
		ChannelID     int
		ChannelType   int
		ChannelStatus int
	}
	err := model.DB.Model(&model.Ability{}).Select("abilities.channel_id, channels.type as channel_type, channels.status as channel_status").Joins("left join channels on channels.id = abilities.channel_id").Where(map[string]any{"abilities.group": group, "abilities.model": modelName, "abilities.enabled": true}).Scan(&abilities).Error
	if err != nil {
		return nil, err
	}
	skip := map[int]struct{}{}
	for _, a := range abilities {
		if a.ChannelStatus != common.ChannelStatusEnabled || !QualityMonitorChannelSupported(a.ChannelType) {
			skip[a.ChannelID] = struct{}{}
		}
	}
	channel, err := model.GetChannelWithOptions(group, modelName, 0, "/v1/responses", model.ChannelSelectionOptions{SkipChannelIDs: skip})
	if err != nil {
		return nil, fmt.Errorf("quality monitor channel selection failed: %w", err)
	}
	if channel != nil && (channel.Status != common.ChannelStatusEnabled || !QualityMonitorChannelSupported(channel.Type)) {
		return nil, nil
	}
	return channel, nil
}
