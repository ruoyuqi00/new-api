package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func qualityMonitorControllerFixture(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QualityMonitorPlan{}, &model.QualityMonitorResult{}, &model.SystemTask{}, &model.SystemTaskLock{}, &model.Token{}, &model.Log{}, &model.QuotaData{}, &model.ChannelAccountPoolBinding{}, &model.AccountPool{}))
	previousGroups := setting.UserUsableGroups2JSONString()
	previousRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"alpha":"Alpha"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"alpha":1,"beta":1}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
	})
	require.NoError(t, db.Create(&model.User{Id: 801, Username: "monitor-admin", AffCode: "monitor-admin", Group: "alpha", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Quota: 123456, UsedQuota: 789}).Error)
	require.NoError(t, db.Create(&model.User{Id: 802, Username: "monitor-reader", AffCode: "monitor-reader", Group: "alpha", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}).Error)
	return db
}

func qualityMonitorContext(method, path, body string, id, role int) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", id)
	ctx.Set("role", role)
	return ctx, recorder
}

func TestQualityMonitorReadPermissionsAndRedaction(t *testing.T) {
	db := qualityMonitorControllerFixture(t)
	published := model.QualityMonitorPlan{Name: "Published", Published: true, Groups: []string{"alpha"}, Models: []string{"gpt-6-astra"}, Questions: []model.QualityMonitorQuestion{}}
	private := model.QualityMonitorPlan{Name: "Private", Groups: []string{"alpha"}, Models: []string{"gpt-6-astra"}, Questions: []model.QualityMonitorQuestion{}}
	require.NoError(t, db.Create(&published).Error)
	require.NoError(t, db.Create(&private).Error)
	rows := []model.QualityMonitorResult{{PlanID: published.ID, Group: "alpha", Model: "gpt-6-astra", Answer: "visible", ChannelID: 81, Error: "upstream secret", ActualResponseModel: "internal", EstimatedQuota: 99}, {PlanID: published.ID, Group: "beta", Answer: "forbidden-group"}, {PlanID: private.ID, Group: "alpha", Answer: "unpublished"}}
	for i := range rows {
		rows[i].CreatedAt = common.GetTimestamp()
	}
	require.NoError(t, db.Create(&rows).Error)
	ctx, recorder := qualityMonitorContext("GET", "/api/quality-monitor/results?page=1&page_size=20", "", 802, common.RoleCommonUser)
	GetQualityMonitorResults(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Items []map[string]any `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, "visible", response.Data.Items[0]["answer"])
	for _, key := range []string{"channel_id", "error", "actual_response_model", "estimated_quota"} {
		assert.NotContains(t, response.Data.Items[0], key)
	}
	// A changed DB group must take effect without relying on stale session/cache.
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 802).Update("group", "beta").Error)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{}`))
	ctx, recorder = qualityMonitorContext("GET", "/api/quality-monitor/results?group=alpha", "", 802, common.RoleCommonUser)
	GetQualityMonitorResults(ctx)
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Zero(t, response.Data.Total)
	ctx, recorder = qualityMonitorContext("GET", "/api/quality-monitor/results", "", 801, common.RoleAdminUser)
	GetQualityMonitorResults(ctx)
	assert.Contains(t, recorder.Body.String(), `"channel_id":81`)
	assert.Contains(t, recorder.Body.String(), "unpublished")
}

func TestQualityMonitorAdminOnlyWritesAndNewPlanIsPaused(t *testing.T) {
	db := qualityMonitorControllerFixture(t)
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: "gpt-6-astra", Group: "alpha", Key: "local"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "alpha", Model: "gpt-6-astra", ChannelId: channel.Id, Enabled: true}).Error)
	body := `{"name":"Monitor","enabled":true,"published":true,"groups":["alpha"],"models":["gpt-6-astra"],"interval_minutes":5,"reasoning_effort":"high","max_output_tokens":512,"timeout_seconds":30,"questions":[{"id":"q1","prompt":"6 times 7?","expected_answer":"42","match_type":"exact"}]}`
	ctx, recorder := qualityMonitorContext("POST", "/api/quality-monitor/plans", body, 802, common.RoleCommonUser)
	CreateQualityMonitorPlan(ctx)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	ctx, recorder = qualityMonitorContext("POST", "/api/quality-monitor/plans", body, 801, common.RoleAdminUser)
	CreateQualityMonitorPlan(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                     `json:"success"`
		Data    model.QualityMonitorPlan `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.False(t, response.Data.Enabled)
	assert.False(t, response.Data.Published)
	ctx, recorder = qualityMonitorContext("POST", "/api/quality-monitor/plans/1/run", "", 801, common.RoleAdminUser)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(response.Data.ID)}}
	RunQualityMonitorPlan(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "systask_")
	ctx, recorder = qualityMonitorContext("POST", "/api/quality-monitor/plans/1/run", "", 801, common.RoleAdminUser)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(response.Data.ID)}}
	RunQualityMonitorPlan(ctx)
	assert.Equal(t, http.StatusConflict, recorder.Code)
	// Disabling the last channel cannot prevent an admin from pausing or
	// unpublishing the existing plan; a new invalid group still gets rejected.
	require.NoError(t, db.Where("channel_id = ?", channel.Id).Delete(&model.Ability{}).Error)
	ctx, recorder = qualityMonitorContext("PUT", "/api/quality-monitor/plans/1", strings.ReplaceAll(body, `true`, `false`), 801, common.RoleAdminUser)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(response.Data.ID)}}
	UpdateQualityMonitorPlan(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)
	ctx, recorder = qualityMonitorContext("PUT", "/api/quality-monitor/plans/1", strings.ReplaceAll(body, `"alpha"`, `"missing"`), 801, common.RoleAdminUser)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(response.Data.ID)}}
	UpdateQualityMonitorPlan(ctx)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestQualityMonitorRetentionRunsWithoutEnabledPlans(t *testing.T) {
	db := qualityMonitorControllerFixture(t)
	require.NoError(t, db.Create(&model.QualityMonitorResult{Answer: "expired", CreatedAt: common.GetTimestamp() - 31*86400}).Error)
	handler := qualityMonitorHandler{}
	require.True(t, handler.Enabled())
	task, err := model.CreateSystemTask(model.SystemTaskTypeQualityMonitor, nil, nil)
	require.NoError(t, err)
	claimed, ok, err := model.ClaimSystemTask(task.ID, model.SystemTaskTypeQualityMonitor, "cleanup", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	handler.Run(context.Background(), claimed, "cleanup")
	var count int64
	require.NoError(t, db.Model(&model.QualityMonitorResult{}).Count(&count).Error)
	assert.Zero(t, count)
	assert.False(t, handler.Enabled())
}

// This fixture exercises the real Responses adapter and current group routing.
// Charging a user/token, writing consume logs/quota_data, ignoring the frozen
// prompt/budget, or reporting the requested model as actual must break it.
func TestQualityMonitorRealAdapterRoutingAnswerAndAccounting(t *testing.T) {
	db := qualityMonitorControllerFixture(t)
	service.InitHttpClient()
	withTieredBillingConfig(t, map[string]string{"gpt-6-astra": "tiered_expr"}, map[string]string{"gpt-6-astra": "p * 2 + c * 8"})
	requests := make(chan map[string]any, 4)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := common.DecodeJson(r.Body, &body); err != nil {
			http.Error(w, "bad input", 400)
			return
		}
		requests <- body
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"resp_local","object":"response","status":"completed","model":"gpt-6-astra-observed","output":[{"id":"msg_local","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"42","annotations":[]}]}],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}`)
	}))
	defer upstream.Close()
	base := upstream.URL
	priority := int64(10)
	selected := model.Channel{Name: "local selected", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "local-test-secret", BaseURL: &base, Models: "gpt-6-astra", Group: "alpha", Priority: &priority}
	selected.ModelMapping = common.GetPointer(`{"gpt-6-astra":"gpt-6-astra-low"}`)
	selected.ParamOverride = common.GetPointer(`{"input":"overridden prompt","max_output_tokens":5,"stream":true,"reasoning":{"effort":"low"},"previous_response_id":"must-not-send"}`)
	require.NoError(t, db.Create(&selected).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "alpha", Model: "gpt-6-astra", ChannelId: selected.Id, Enabled: true, Priority: &priority, Weight: 100}).Error)
	lowerPriority := int64(1)
	other := selected
	other.Id = 0
	other.Name = "must not select"
	other.BaseURL = common.GetPointer("http://127.0.0.1:1")
	other.Priority = &lowerPriority
	require.NoError(t, db.Create(&other).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "alpha", Model: "gpt-6-astra", ChannelId: other.Id, Enabled: true, Priority: &lowerPriority, Weight: 100}).Error)
	token := model.Token{UserId: 801, Key: "accounting-token", RemainQuota: 1000, UsedQuota: 10}
	require.NoError(t, db.Create(&token).Error)
	plan := model.QualityMonitorPlan{Name: "Adapter", Groups: []string{"alpha"}, Models: []string{"gpt-6-astra"}, IntervalMinutes: 1, ReasoningEffort: "high", MaxOutputTokens: 512, TimeoutSeconds: 30, Questions: []model.QualityMonitorQuestion{{ID: "q1", Prompt: "What is six times seven?", ExpectedAnswer: "42", MatchType: "exact"}}}
	require.NoError(t, model.SaveQualityMonitorPlan(&plan, true))
	task, err := service.StartQualityMonitorRun(plan.ID)
	require.NoError(t, err)
	claimed, ok, err := model.ClaimSystemTask(task.ID, model.SystemTaskTypeQualityMonitor, "test-runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, service.RunQualityMonitorTask(context.Background(), claimed, "test-runner", probeQualityMonitorChannel))
	require.Len(t, requests, 1)
	sent := <-requests
	assert.Equal(t, "What is six times seven?", sent["input"])
	assert.EqualValues(t, 512, sent["max_output_tokens"])
	assert.Equal(t, false, sent["stream"])
	assert.Equal(t, false, sent["store"])
	assert.Equal(t, map[string]any{"effort": "high"}, sent["reasoning"])
	assert.NotContains(t, sent, "previous_response_id")
	items, _, err := model.ListQualityMonitorResults(model.QualityMonitorResultFilter{Admin: true, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "42", items[0].Answer)
	assert.Equal(t, "passed", items[0].Status)
	assert.Equal(t, selected.Id, items[0].ChannelID)
	assert.Equal(t, "gpt-6-astra-observed", items[0].ActualResponseModel)
	assert.Equal(t, 18, items[0].EstimatedQuota)
	var user model.User
	require.NoError(t, db.First(&user, 801).Error)
	assert.Equal(t, 123456, user.Quota)
	assert.Equal(t, 789, user.UsedQuota)
	var afterToken model.Token
	require.NoError(t, db.First(&afterToken, token.Id).Error)
	assert.Equal(t, 1000, afterToken.RemainQuota)
	assert.Equal(t, 10, afterToken.UsedQuota)
	for _, table := range []any{&model.Log{}, &model.QuotaData{}} {
		var count int64
		require.NoError(t, db.Model(table).Count(&count).Error)
		assert.Zero(t, count)
	}
	var afterChannel model.Channel
	require.NoError(t, db.First(&afterChannel, selected.Id).Error)
	assert.Zero(t, afterChannel.UsedQuota)
	// Ordinary channel tests retain their existing consume-log behavior/default hi.
	logEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = logEnabled })
	selected.ParamOverride = nil
	ordinary := testChannel(context.Background(), &selected, 801, "gpt-6-astra", string(constant.EndpointTypeOpenAIResponse), false, "")
	require.NoError(t, ordinary.localErr)
	assert.Equal(t, "42", ordinary.responseContent)
	var count int64
	require.NoError(t, db.Model(&model.Log{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestQualityMonitorMalformedResponseAndAnswerLimit(t *testing.T) {
	qualityMonitorControllerFixture(t)
	service.InitHttpClient()
	withTieredBillingConfig(t, map[string]string{"gpt-6-astra": "tiered_expr"}, map[string]string{"gpt-6-astra": "p * 2 + c * 8"})
	cases := []struct {
		name, body         string
		wantErr, truncated bool
		length             int
	}{
		{name: "malformed", body: `not json`, wantErr: true},
		{name: "invalid response shape", body: `{"unrelated":"data"}`, wantErr: true},
		{name: "failed upstream response", body: `{"id":"resp_bad","object":"response","status":"failed","error":{"message":"private secret","code":"server_error"}}`, wantErr: true},
		{name: "bounded answer", body: `{"id":"resp_long","object":"response","status":"completed","model":"actual","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"` + strings.Repeat("a", 65537) + `"}]}],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}`, truncated: true, length: 65535},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tc.body)
			}))
			defer upstream.Close()
			base := upstream.URL
			channel := &model.Channel{Type: constant.ChannelTypeOpenAI, Key: "local-test-secret", BaseURL: &base}
			response, err := probeQualityMonitorChannel(context.Background(), channel, model.QualityMonitorPlan{Groups: []string{"alpha"}, Models: []string{"gpt-6-astra"}, ReasoningEffort: "high", MaxOutputTokens: 512, TimeoutSeconds: 30}, model.QualityMonitorQuestion{Prompt: "Answer"})
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.length, len(response.Answer))
			assert.Equal(t, tc.truncated, response.AnswerTruncated)
		})
	}
}
