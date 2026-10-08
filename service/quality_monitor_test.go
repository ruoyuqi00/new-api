package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func qualityMonitorFixture(t *testing.T) model.QualityMonitorPlan {
	t.Helper()
	db := setupYucoreMediaCatalogTest(t)
	require.NoError(t, db.AutoMigrate(&model.QualityMonitorPlan{}, &model.QualityMonitorResult{}, &model.SystemTask{}, &model.SystemTaskLock{}))
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Name: "alpha", Key: "secret", Status: common.ChannelStatusEnabled, Models: "gpt-6-astra", Group: "alpha"}
	require.NoError(t, db.Create(&channel).Error)
	priority := int64(10)
	require.NoError(t, db.Create(&model.Ability{Group: "alpha", Model: "gpt-6-astra", ChannelId: channel.Id, Enabled: true, Priority: &priority, Weight: 100}).Error)
	return model.QualityMonitorPlan{Name: "Quality", Groups: []string{"alpha"}, Models: []string{"gpt-6-astra"}, IntervalMinutes: 1, ReasoningEffort: "high", MaxOutputTokens: 512, TimeoutSeconds: 30, Questions: []model.QualityMonitorQuestion{{ID: "q1", Prompt: "What is six times seven?", ExpectedAnswer: "42", MatchType: "exact"}}}
}

func TestQualityMonitorMaximumQuestionSetManualEnqueueRoundTrip(t *testing.T) {
	plan := qualityMonitorFixture(t)
	plan.Questions = nil
	for i := 0; i < 5; i++ {
		plan.Questions = append(plan.Questions, model.QualityMonitorQuestion{ID: fmt.Sprintf("q%d", i), Prompt: strings.Repeat("p", 8192), ExpectedAnswer: strings.Repeat("e", 8192), MatchType: "exact"})
	}
	require.NoError(t, ValidateQualityMonitorPlan(&plan))
	require.NoError(t, model.SaveQualityMonitorPlan(&plan, true))
	task, err := StartQualityMonitorRun(plan.ID)
	require.NoError(t, err)
	persisted, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	assert.Greater(t, len(persisted.Payload), 65535)
	var payload qualityMonitorTaskPayload
	require.NoError(t, persisted.DecodePayload(&payload))
	require.NotNil(t, payload.Plan)
	assert.Equal(t, plan.Questions, payload.Plan.Questions)
	assert.Equal(t, plan.Groups, payload.Plan.Groups)
	assert.Equal(t, plan.Models, payload.Plan.Models)
	assert.Equal(t, "high", payload.Plan.ReasoningEffort)
}

func TestQualityMonitorTruncatedAnswerCannotAutoPass(t *testing.T) {
	plan := qualityMonitorFixture(t)
	require.NoError(t, model.SaveQualityMonitorPlan(&plan, true))
	task, err := StartQualityMonitorRun(plan.ID)
	require.NoError(t, err)
	claimed, ok, err := model.ClaimSystemTask(task.ID, model.SystemTaskTypeQualityMonitor, "runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, RunQualityMonitorTask(context.Background(), claimed, "runner", func(context.Context, *model.Channel, model.QualityMonitorPlan, model.QualityMonitorQuestion) (QualityMonitorProbeResult, error) {
		return QualityMonitorProbeResult{Answer: "42" + strings.Repeat(" ", 65534) + "WRONG"}, nil
	}))
	items, _, err := model.ListQualityMonitorResults(model.QualityMonitorResultFilter{Admin: true, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.True(t, items[0].AnswerTruncated)
	assert.Equal(t, "failed", items[0].Status)
}

func TestQualityMonitorCodexAndDisabledChannelsAreSkipped(t *testing.T) {
	qualityMonitorFixture(t)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("name = ?", "alpha").Update("type", constant.ChannelTypeCodex).Error)
	channel, err := SelectQualityMonitorChannel("alpha", "gpt-6-astra")
	require.NoError(t, err)
	assert.Nil(t, channel)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("name = ?", "alpha").Update("type", constant.ChannelTypeOpenAI).Update("status", common.ChannelStatusManuallyDisabled).Error)
	channel, err = SelectQualityMonitorChannel("alpha", "gpt-6-astra")
	require.NoError(t, err)
	assert.Nil(t, channel)
}

func TestQualityMonitorDisabledPriorityDoesNotHideActiveFallback(t *testing.T) {
	qualityMonitorFixture(t)
	priority := int64(100)
	disabled := model.Channel{Name: "disabled", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusManuallyDisabled, Key: "unused", Models: "gpt-6-astra", Group: "alpha"}
	require.NoError(t, model.DB.Create(&disabled).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "alpha", Model: "gpt-6-astra", ChannelId: disabled.Id, Enabled: true, Priority: &priority, Weight: 100}).Error)
	channel, err := SelectQualityMonitorChannel("alpha", "gpt-6-astra")
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, "alpha", channel.Name)
}

func TestQualityMonitorSnapshotAndLostLeaseCannotSubmitNextQuestion(t *testing.T) {
	plan := qualityMonitorFixture(t)
	plan.Questions = append(plan.Questions, model.QualityMonitorQuestion{ID: "q2", Prompt: "second", MatchType: "manual"})
	require.NoError(t, model.SaveQualityMonitorPlan(&plan, true))
	plan.Enabled = true
	require.NoError(t, model.SaveQualityMonitorPlan(&plan, false))
	task, err := StartQualityMonitorRun(plan.ID)
	require.NoError(t, err)
	plan.Questions[0].Prompt = "edited after enqueue"
	require.NoError(t, model.SaveQualityMonitorPlan(&plan, false))
	claimed, ok, err := model.ClaimSystemTask(task.ID, model.SystemTaskTypeQualityMonitor, "runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	calls := 0
	err = RunQualityMonitorTask(context.Background(), claimed, "runner", func(ctx context.Context, ch *model.Channel, snapshot model.QualityMonitorPlan, q model.QualityMonitorQuestion) (QualityMonitorProbeResult, error) {
		calls++
		assert.Equal(t, "What is six times seven?", q.Prompt)
		require.NoError(t, model.DB.Model(&model.SystemTaskLock{}).Where("task_id = ?", task.TaskID).Update("locked_until", common.GetTimestamp()-1).Error)
		require.NoError(t, model.ExpireStaleSystemTaskLocks(common.GetTimestamp()))
		return QualityMonitorProbeResult{Answer: "42"}, nil
	})
	require.ErrorIs(t, err, model.ErrSystemTaskLockLost)
	assert.Equal(t, 1, calls)
	stored, err := model.GetQualityMonitorPlan(plan.ID)
	require.NoError(t, err)
	// The failed worker must preserve the conservative full-budget reservation.
	assert.GreaterOrEqual(t, stored.NextRunAt, stored.LastRunAt+120)
	persisted, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	assert.Equal(t, model.SystemTaskStatusFailed, persisted.Status)
	items, _, err := model.ListQualityMonitorResults(model.QualityMonitorResultFilter{Admin: true, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "What is six times seven?", items[0].Prompt)
}

func TestQualityMonitorDiagnosticPreservesStageWithoutSensitiveMessage(t *testing.T) {
	plan := qualityMonitorFixture(t)
	require.NoError(t, model.SaveQualityMonitorPlan(&plan, true))
	task, err := StartQualityMonitorRun(plan.ID)
	require.NoError(t, err)
	claimed, ok, err := model.ClaimSystemTask(task.ID, model.SystemTaskTypeQualityMonitor, "runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, RunQualityMonitorTask(context.Background(), claimed, "runner", func(context.Context, *model.Channel, model.QualityMonitorPlan, model.QualityMonitorQuestion) (QualityMonitorProbeResult, error) {
		return QualityMonitorProbeResult{}, types.NewError(errors.New("Bearer secret-private-key"), types.ErrorCodeModelPriceError)
	}))
	items, _, err := model.ListQualityMonitorResults(model.QualityMonitorResultFilter{Admin: true, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Model pricing is not configured or invalid", items[0].Error)
}

func TestQualityMonitorRejectsUnsafeConfigurations(t *testing.T) {
	plan := qualityMonitorFixture(t)
	require.NoError(t, ValidateQualityMonitorPlan(&plan))
	cases := []struct {
		name   string
		change func(*model.QualityMonitorPlan)
	}{
		{"unsupported model", func(p *model.QualityMonitorPlan) { p.Models = []string{"gpt-4o"} }},
		{"zero interval", func(p *model.QualityMonitorPlan) { p.IntervalMinutes = 0 }},
		{"excessive interval", func(p *model.QualityMonitorPlan) { p.IntervalMinutes = 10081 }},
		{"duplicate ids", func(p *model.QualityMonitorPlan) { p.Questions = append(p.Questions, p.Questions[0]) }},
		{"too many calls", func(p *model.QualityMonitorPlan) {
			p.Groups = make([]string, 20)
			p.Models = []string{"gpt-6-astra", "gpt-6.1-sol"}
			p.Questions = make([]model.QualityMonitorQuestion, 5)
		}},
		{"unknown group", func(p *model.QualityMonitorPlan) { p.Groups = []string{"missing"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { p := plan; tc.change(&p); require.Error(t, ValidateQualityMonitorPlan(&p)) })
	}
}

func TestQualityMonitorAnswerDecisions(t *testing.T) {
	cases := []struct{ kind, answer, expected, want string }{
		{"exact", "  FORTY TWO ", "forty two", "passed"}, {"exact", "42", "43", "failed"},
		{"contains", "The answer is Forty Two.", "forty two", "passed"}, {"contains", "", "", "failed"},
		{"manual", "42", "", "ungraded"}, {"exact", "  ", "", "failed"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, EvaluateQualityMonitorAnswer(tc.kind, tc.answer, tc.expected))
	}
}

func TestQualityMonitorManualRunPausedAndGlobalConflict(t *testing.T) {
	plan := qualityMonitorFixture(t)
	require.NoError(t, model.SaveQualityMonitorPlan(&plan, true))
	task, err := StartQualityMonitorRun(plan.ID)
	require.NoError(t, err)
	_, err = StartQualityMonitorRun(plan.ID)
	require.ErrorIs(t, err, ErrQualityMonitorBusy)
	claimed, ok, err := model.ClaimSystemTask(task.ID, model.SystemTaskTypeQualityMonitor, "runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	_, ok, err = model.ClaimSystemTask(task.ID, model.SystemTaskTypeQualityMonitor, "other", common.GetTimestamp()+60)
	require.NoError(t, err)
	assert.False(t, ok)
	err = RunQualityMonitorTask(context.Background(), claimed, "runner", func(context.Context, *model.Channel, model.QualityMonitorPlan, model.QualityMonitorQuestion) (QualityMonitorProbeResult, error) {
		return QualityMonitorProbeResult{Answer: "42", ActualResponseModel: "gpt-6-astra-actual", EstimatedQuota: 5}, nil
	})
	require.NoError(t, err)
	stored, err := model.GetQualityMonitorPlan(plan.ID)
	require.NoError(t, err)
	assert.False(t, stored.Enabled)
	assert.Zero(t, stored.NextRunAt)
	items, total, err := model.ListQualityMonitorResults(model.QualityMonitorResultFilter{Admin: true, Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, "passed", items[0].Status)
	assert.Equal(t, "42", items[0].Answer)
}

func TestQualityMonitorFailedScheduleAdvancesAndUnsupportedPairSkips(t *testing.T) {
	plan := qualityMonitorFixture(t)
	plan.Models = append(plan.Models, "gpt-6.1-sol")
	require.NoError(t, model.SaveQualityMonitorPlan(&plan, true))
	plan.Enabled = true
	require.NoError(t, model.SaveQualityMonitorPlan(&plan, false))
	require.True(t, HasDueQualityMonitorPlans(common.GetTimestamp()))
	task, err := model.CreateSystemTask(model.SystemTaskTypeQualityMonitor, nil, nil)
	require.NoError(t, err)
	claimed, ok, err := model.ClaimSystemTask(task.ID, model.SystemTaskTypeQualityMonitor, "runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	err = RunQualityMonitorTask(context.Background(), claimed, "runner", func(context.Context, *model.Channel, model.QualityMonitorPlan, model.QualityMonitorQuestion) (QualityMonitorProbeResult, error) {
		return QualityMonitorProbeResult{}, errors.New("upstream failed with secret")
	})
	require.NoError(t, err)
	assert.False(t, HasDueQualityMonitorPlans(common.GetTimestamp()))
	items, _, err := model.ListQualityMonitorResults(model.QualityMonitorResultFilter{Admin: true, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "skipped", items[0].Status)
	assert.Equal(t, "error", items[1].Status)
	assert.NotContains(t, items[1].Error, "secret")
}
