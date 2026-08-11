package relay

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type responseWritingTaskAdaptor struct {
	channel.TaskAdaptor
	taskID string
}

func (a *responseWritingTaskAdaptor) DoResponse(c *gin.Context, _ *http.Response, _ *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	c.JSON(http.StatusOK, map[string]any{"task_id": "public"})
	return a.taskID, nil, nil
}

func TestTaskResponseIsCommittedOnlyAfterTaskIDValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	invalidRecorder := httptest.NewRecorder()
	invalidContext, _ := gin.CreateTestContext(invalidRecorder)
	_, _, taskErr := doValidatedTaskResponse(invalidContext, &responseWritingTaskAdaptor{}, &http.Response{}, &relaycommon.RelayInfo{})
	require.NotNil(t, taskErr)
	assert.Equal(t, dto.TaskSubmissionAccepted, taskErr.SubmissionState())
	assert.Empty(t, invalidRecorder.Body.String())

	validRecorder := httptest.NewRecorder()
	validContext, _ := gin.CreateTestContext(validRecorder)
	taskID, _, taskErr := doValidatedTaskResponse(validContext, &responseWritingTaskAdaptor{taskID: "upstream-id"}, &http.Response{}, &relaycommon.RelayInfo{})
	require.Nil(t, taskErr)
	assert.Equal(t, "upstream-id", taskID)
	assert.JSONEq(t, `{"task_id":"public"}`, validRecorder.Body.String())
}

func TestRejectedRetryThenPreWriteFailureIsNotAmbiguous(t *testing.T) {
	info := &relaycommon.TaskRelayInfo{RequestWritten: true}
	assert.Equal(t, dto.TaskSubmissionRejected, classifyTaskSubmissionState(info.RequestWritten, http.StatusTooManyRequests, false))

	info.BeginRequestAttempt()
	assert.False(t, info.RequestWritten)
	secondError := (&dto.TaskError{StatusCode: http.StatusInternalServerError}).WithSubmissionState(
		classifyTaskSubmissionState(info.RequestWritten, 0, false),
	)
	assert.Equal(t, dto.TaskSubmissionNotSent, secondError.SubmissionState())
	assert.True(t, service.ShouldRefundTaskSubmission(secondError))
}

func TestTaskSubmissionClassification(t *testing.T) {
	tests := []struct {
		name       string
		wrote      bool
		statusCode int
		accepted   bool
		want       dto.TaskSubmissionState
	}{
		{name: "transport before write", want: dto.TaskSubmissionNotSent},
		{name: "explicit rejection", wrote: true, statusCode: http.StatusUnprocessableEntity, want: dto.TaskSubmissionRejected},
		{name: "timeout after write", wrote: true, statusCode: http.StatusRequestTimeout, want: dto.TaskSubmissionAmbiguous},
		{name: "server error after write", wrote: true, statusCode: http.StatusBadGateway, want: dto.TaskSubmissionAmbiguous},
		{name: "unparseable success", wrote: true, statusCode: http.StatusOK, accepted: true, want: dto.TaskSubmissionAccepted},
		{name: "unspecified response after write", wrote: true, statusCode: http.StatusTeapot, want: dto.TaskSubmissionAmbiguous},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, classifyTaskSubmissionState(tt.wrote, tt.statusCode, tt.accepted))
		})
	}
}

func TestTaskCatalogPricingProducesExactFrozenQuota(t *testing.T) {
	groupRatio := 1.2
	perCall, clamp := common.QuotaFromFloatChecked(0.5 * common.QuotaPerUnit * groupRatio)
	require.Nil(t, clamp)
	perCallInfo := &relaycommon.RelayInfo{PriceData: types.PriceData{Quota: perCall, OtherRatios: map[string]float64{"seconds": 99}}}
	resolveTaskPerCallBilling(perCallInfo, "veo-3.1-fast")
	perCallQuota := perCallInfo.PriceData.Quota
	if !perCallInfo.TaskPerCallBilling {
		perCallQuota = applyTaskOtherRatiosQuota(perCallQuota, perCallInfo.PriceData.OtherRatios)
	}
	assert.Equal(t, 300_000, perCallQuota)

	perSecondBase, clamp := common.QuotaFromFloatChecked(0.35 * common.QuotaPerUnit * groupRatio)
	require.Nil(t, clamp)
	perSecond := applyTaskOtherRatiosQuota(perSecondBase, map[string]float64{"seconds": 5})
	assert.Equal(t, 1_050_000, perSecond)
	assert.GreaterOrEqual(t, perSecond, int(0.35*5*common.QuotaPerUnit))
}

func TestApplyTaskOtherRatiosQuota(t *testing.T) {
	require.Equal(t, 2000, applyTaskOtherRatiosQuota(1000, map[string]float64{
		"seconds": 2,
		"size":    1,
	}))
	require.Equal(t, math.MaxInt32, applyTaskOtherRatiosQuota(2000, map[string]float64{
		"seconds": 1.8446744073686647e19,
	}))

	quota, clamp := applyTaskOtherRatiosQuotaChecked(1000, map[string]float64{"seconds": 2})
	require.Equal(t, 2000, quota)
	require.Nil(t, clamp)

	quota, clamp = applyTaskOtherRatiosQuotaChecked(2000, map[string]float64{
		"seconds": 1.8446744073686647e19,
	})
	require.Equal(t, math.MaxInt32, quota)
	require.NotNil(t, clamp)
	require.Equal(t, "overflow", clamp.Kind)
}

func TestRecalcQuotaFromRatiosSaturates(t *testing.T) {
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{
			Quota:       6000,
			OtherRatios: map[string]float64{"seconds": 3},
		},
	}

	require.Equal(t, 4000, recalcQuotaFromRatios(info, map[string]float64{
		"seconds": 2,
	}))
	require.Equal(t, math.MaxInt32, recalcQuotaFromRatios(info, map[string]float64{
		"seconds": 1.8446744073686647e19,
	}))

	quota, clamp := recalcQuotaFromRatiosChecked(info, map[string]float64{
		"seconds": 1.8446744073686647e19,
	})
	require.Equal(t, math.MaxInt32, quota)
	require.NotNil(t, clamp)
	require.Equal(t, "overflow", clamp.Kind)
}

func TestNoteTaskQuotaClampStoresFirstOnly(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	_, first := applyTaskOtherRatiosQuotaChecked(2000, map[string]float64{
		"seconds": 1.8446744073686647e19,
	})
	_, second := applyTaskOtherRatiosQuotaChecked(-2000, map[string]float64{
		"seconds": 1.8446744073686647e19,
	})

	noteTaskQuotaClamp(info, first, "first_op")
	noteTaskQuotaClamp(info, second, "second_op")

	require.NotNil(t, info.TaskQuotaClamp)
	require.Equal(t, "first_op", info.TaskQuotaClamp.Op)
	require.Equal(t, "overflow", info.TaskQuotaClamp.Kind)
}

func TestResolveTaskPerCallBillingSnapshotsConfiguredPricing(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.model_capabilities": `{"snapshot-video":{"kind":"video","pricing_unit":"per_second"}}`,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})

	info := &relaycommon.RelayInfo{PriceData: types.PriceData{UsePrice: true, Quota: 1000, OtherRatios: map[string]float64{"seconds": 3}}}
	resolveTaskPerCallBilling(info, "snapshot-video")
	require.True(t, info.TaskPricingResolved)
	assert.False(t, info.TaskPerCallBilling)

	common.OptionMapRWMutex.Lock()
	common.OptionMap["yucore_media.model_capabilities"] = `{"snapshot-video":{"kind":"video","pricing_unit":"per_call"}}`
	common.OptionMapRWMutex.Unlock()
	resolveTaskPerCallBilling(info, "snapshot-video")
	assert.False(t, info.TaskPerCallBilling, "retry must retain the first pricing decision")

	quota := info.PriceData.Quota
	if !info.TaskPerCallBilling {
		quota = applyTaskOtherRatiosQuota(quota, info.PriceData.OtherRatios)
	}
	assert.Equal(t, 3000, quota)
	assert.False(t, model.TaskBillingContext{PerCallBilling: info.TaskPerCallBilling}.PerCallBilling)
}

func TestTaskModel2UserDtoHidesMappedModelDetails(t *testing.T) {
	originalData := json.RawMessage(`{
		"model":"internal-upstream-model",
		"response":{
			"model":"internal-upstream-model",
			"text":"internal-upstream-model remains ordinary user content",
			"request_id":9007199254740993
		},
		"prompt":"render with internal-upstream-model in the caption",
		"metadata":{"model":"internal-upstream-model"},
		"request_id":9007199254740993
	}`)
	task := &model.Task{
		Properties: model.Properties{
			Input:             "user input",
			UpstreamModelName: "internal-upstream-model",
			OriginModelName:   "public-model",
		},
		Data: append(json.RawMessage(nil), originalData...),
	}

	userDto := TaskModel2UserDto(task)
	userJSON, err := common.Marshal(userDto)
	require.NoError(t, err)
	var userPayload struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Data       map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, common.Unmarshal(userJSON, &userPayload))

	assert.NotContains(t, userPayload.Properties, "upstream_model_name")
	assert.JSONEq(t, `"public-model"`, string(userPayload.Properties["origin_model_name"]))
	assert.JSONEq(t, `"user input"`, string(userPayload.Properties["input"]))
	assert.JSONEq(t, `"public-model"`, string(userPayload.Data["model"]))
	assert.JSONEq(t, `"render with internal-upstream-model in the caption"`, string(userPayload.Data["prompt"]))
	assert.JSONEq(t, `{"model":"internal-upstream-model"}`, string(userPayload.Data["metadata"]))
	assert.Equal(t, "9007199254740993", string(userPayload.Data["request_id"]))

	var response map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(userPayload.Data["response"], &response))
	assert.JSONEq(t, `"public-model"`, string(response["model"]))
	assert.JSONEq(t, `"internal-upstream-model remains ordinary user content"`, string(response["text"]))
	assert.Equal(t, "9007199254740993", string(response["request_id"]))

	assert.Equal(t, "internal-upstream-model", task.Properties.UpstreamModelName)
	assert.Equal(t, string(originalData), string(task.Data))

	adminDto := TaskModel2Dto(task)
	adminJSON, err := common.Marshal(adminDto)
	require.NoError(t, err)
	var adminPayload struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Data       json.RawMessage            `json:"data"`
	}
	require.NoError(t, common.Unmarshal(adminJSON, &adminPayload))
	assert.JSONEq(t, `"internal-upstream-model"`, string(adminPayload.Properties["upstream_model_name"]))
	assert.JSONEq(t, string(originalData), string(adminPayload.Data))
}
