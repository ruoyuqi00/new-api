package xai

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestConvertToXAIRequestUsesImagineVideoFields(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video-1.5",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-video-1.5"},
	}
	req := relaycommon.TaskSubmitReq{
		Prompt:  "a paper boat crossing a rainy city",
		Seconds: "6",
		Size:    "1280x720",
	}

	payload, err := adaptor.convertToRequestPayload(&req, info)

	require.NoError(t, err)
	require.Equal(t, "grok-imagine-video-1.5", payload.Model)
	require.Equal(t, "a paper boat crossing a rainy city", payload.Prompt)
	require.Equal(t, 6, payload.Duration)
	require.Equal(t, "16:9", payload.AspectRatio)
}

func TestParseTaskResultMapsXAIStatusesAndVideoURL(t *testing.T) {
	adaptor := &TaskAdaptor{}

	result, err := adaptor.ParseTaskResult([]byte(`{"request_id":"req_123","status":"done","video":{"url":"https://cdn.example/video.mp4"}}`))

	require.NoError(t, err)
	require.Equal(t, model.TaskStatusSuccess, result.Status)
	require.Equal(t, "https://cdn.example/video.mp4", result.Url)
	require.Equal(t, "100%", result.Progress)
}

func TestParseTaskResultTreatsBothCanceledSpellingsAsFailure(t *testing.T) {
	for _, status := range []string{"canceled", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{"status":"` + status + `"}`))
			require.NoError(t, err)
			require.Equal(t, model.TaskStatusFailure, result.Status)
		})
	}
}

func TestParseTaskResultResolvesRelativeVideoURL(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://api.example.com"}

	result, err := adaptor.ParseTaskResult([]byte(`{"status":"done","video":{"url":"/v1/videos/req_123/content"}}`))

	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/v1/videos/req_123/content", result.Url)
}

func TestGetModelListOnlyContainsImagineMediaModels(t *testing.T) {
	models := (&TaskAdaptor{}).GetModelList()

	require.ElementsMatch(t, []string{
		"grok-imagine-image",
		"grok-imagine-image-quality",
		"grok-imagine-video",
		"grok-imagine-video-1.5",
		"grok-imagine-video-1.5-preview",
	}, models)
}

func TestGrokImagineVideoBillingUsesSecondsAndResolution(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		seconds    string
		size       string
		wantSecond float64
		wantRes    float64
	}{
		{name: "default", model: "grok-imagine-video", wantSecond: 5, wantRes: 1},
		{name: "720p", model: "grok-imagine-video-1.5", seconds: "10", size: "1280x720", wantSecond: 10, wantRes: 0.0594 / 0.0414},
		{name: "1080p preview", model: "grok-imagine-video-1.5-preview", seconds: "15", size: "1920x1080", wantSecond: 15, wantRes: 0.0774 / 0.0414},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratios, err := grokImagineVideoBilling(relaycommon.TaskSubmitReq{
				Model:   tt.model,
				Seconds: tt.seconds,
				Size:    tt.size,
			})

			require.NoError(t, err)
			require.InDelta(t, tt.wantSecond, ratios["seconds"], 0.000000001)
			require.InDelta(t, tt.wantRes, ratios["resolution"], 0.000000001)
		})
	}
}

func TestGrokImagineVideoBillingUsesMetadataOverrides(t *testing.T) {
	ratios, err := grokImagineVideoBilling(relaycommon.TaskSubmitReq{
		Model:    "grok-imagine-video",
		Seconds:  "5",
		Size:     "1280x720",
		Metadata: map[string]interface{}{"duration": 7, "resolution": "1080p"},
	})

	require.NoError(t, err)
	require.InDelta(t, 7, ratios["seconds"], 0.000000001)
	require.InDelta(t, 0.0774/0.0414, ratios["resolution"], 0.000000001)
}

func TestGrokImagineVideoBillingRejectsUnsupportedDimensions(t *testing.T) {
	for _, req := range []relaycommon.TaskSubmitReq{
		{Model: "grok-imagine-video", Seconds: "16"},
		{Model: "grok-imagine-video", Seconds: "-1"},
		{Model: "grok-imagine-video", Seconds: "0"},
		{Model: "grok-imagine-video", Size: "1024x1024"},
		{Model: "grok-imagine-video", Metadata: map[string]interface{}{"resolution": "4k"}},
		{Model: "grok-imagine-video", Metadata: map[string]interface{}{"resolution": "fooX720"}},
	} {
		_, err := grokImagineVideoBilling(req)
		require.Error(t, err)
	}
}

func TestGrokImagineVideoBillingRejectsExplicitDurationZero(t *testing.T) {
	for _, requestJSON := range []string{
		`{"model":"grok-imagine-video","duration":0}`,
		`{"model":"grok-imagine-video","seconds":0}`,
		`{"model":"grok-imagine-video","duration":5,"seconds":0}`,
	} {
		request := relaycommon.TaskSubmitReq{}
		require.NoError(t, common.Unmarshal([]byte(requestJSON), &request))
		_, err := grokImagineVideoBilling(request)
		require.Error(t, err)
	}
}

func TestGrokImagineVideoMetadataResolutionIsCanonicalized(t *testing.T) {
	request := relaycommon.TaskSubmitReq{
		Model:    "grok-imagine-video",
		Seconds:  "5",
		Metadata: map[string]interface{}{"resolution": "1280x720"},
	}
	dimensions, err := normalizeGrokImagineVideoDimensions(request)
	require.NoError(t, err)
	assert.Equal(t, "720p", dimensions.Resolution)
}

func TestValidateGrokImagineVideoRejectsBeforeSubmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(`{
		"model":"grok-imagine-video",
		"prompt":"test",
		"seconds":16,
		"size":"1280x720"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
	}

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(context, info)

	require.NotNil(t, taskErr)
	require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	require.True(t, taskErr.LocalError)
}
