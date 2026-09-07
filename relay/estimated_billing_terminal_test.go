package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEmitEstimatedBillingTerminalForAmbiguousResponsesStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		IsStream:        true,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeResponses,
		OriginModelName: "gpt-test",
		Request: &dto.OpenAIResponsesRequest{
			Model:  "gpt-test",
			Stream: common.GetPointer(true),
		},
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	info.SetEstimatePromptTokens(1200)
	attempt := info.BeginUpstreamRequestAttempt()
	attempt.MarkRequestWritten()
	attempt.MarkAmbiguousIfPotentiallySent()
	info.PreservePreConsumedQuota = true

	emitted, err := EmitEstimatedBillingTerminal(c, info)

	require.NoError(t, err)
	require.True(t, emitted)
	require.True(t, info.StreamEstimatedTerminalSent)
	require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Body.String(), "event: response.incomplete")
	require.Contains(t, recorder.Body.String(), `"input_tokens":1200`)
	require.Contains(t, recorder.Body.String(), `"output_tokens":0`)
	require.Contains(t, recorder.Body.String(), `"total_tokens":1200`)
}

func TestEmitEstimatedBillingTerminalSkipsMedia(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	info := &relaycommon.RelayInfo{
		IsStream:        true,
		RelayFormat:     types.RelayFormatOpenAIImage,
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		OriginModelName: "gpt-image-2",
	}

	emitted, err := EmitEstimatedBillingTerminal(c, info)

	require.NoError(t, err)
	require.False(t, emitted)
	require.Empty(t, recorder.Body.String())
}

func TestEmitEstimatedBillingTerminalSkipsResponsesImageGenerationTool(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		IsStream:                 true,
		RelayFormat:              types.RelayFormatOpenAIResponses,
		RelayMode:                relayconstant.RelayModeResponses,
		OriginModelName:          "gpt-test",
		PreservePreConsumedQuota: true,
		ChannelMeta:              &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
		Request: &dto.OpenAIResponsesRequest{
			Model:  "gpt-test",
			Stream: common.GetPointer(true),
			Tools:  []byte(`[{"type":"image_generation"}]`),
		},
	}
	attempt := info.BeginUpstreamRequestAttempt()
	attempt.MarkRequestWritten()
	attempt.MarkAmbiguousIfPotentiallySent()

	emitted, err := EmitEstimatedBillingTerminal(c, info)

	require.NoError(t, err)
	require.False(t, emitted)
	require.Empty(t, recorder.Body.String())
}

func TestEmitEstimatedBillingTerminalSkipsNonOpenAITextFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		IsStream:                 true,
		RelayFormat:              types.RelayFormatClaude,
		RelayMode:                relayconstant.RelayModeChatCompletions,
		OriginModelName:          "claude-test",
		PreservePreConsumedQuota: true,
		ChannelMeta:              &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"},
	}
	attempt := info.BeginUpstreamRequestAttempt()
	attempt.MarkRequestWritten()
	attempt.MarkAmbiguousIfPotentiallySent()

	emitted, err := EmitEstimatedBillingTerminal(c, info)

	require.NoError(t, err)
	require.False(t, emitted)
	require.Empty(t, recorder.Header().Get("Content-Type"))
	require.Empty(t, recorder.Body.String())
}
