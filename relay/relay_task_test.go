package relay

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
