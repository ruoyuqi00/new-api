package xai

import (
	"testing"

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
