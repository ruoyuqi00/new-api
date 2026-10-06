package gemini

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiNonStreamHandlersCaptureReturnedModelBeforeConversion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const payload = `{"modelVersion":"gemini-upstream-version","candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":3,"totalTokenCount":5}}`

	for _, tt := range []struct {
		name   string
		format types.RelayFormat
		path   string
		handle func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError)
	}{
		{name: "native generateContent", format: types.RelayFormatGemini, path: "/v1beta/models/gemini-public:generateContent", handle: GeminiTextGenerationHandler},
		{name: "OpenAI chat conversion", format: types.RelayFormatOpenAI, path: "/v1/chat/completions", handle: GeminiChatHandler},
		{name: "Claude messages conversion", format: types.RelayFormatClaude, path: "/v1/messages", handle: GeminiChatHandler},
		{name: "Responses conversion", format: types.RelayFormatOpenAIResponses, path: "/v1/responses", handle: GeminiResponsesHandler},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, nil)
			c.Set(common.RequestIdKey, "gemini-model-audit")
			info := &relaycommon.RelayInfo{
				RelayFormat:     tt.format,
				OriginModelName: "gemini-public",
				ChannelMeta: &relaycommon.ChannelMeta{
					IsModelMapped:     true,
					UpstreamModelName: "gemini-forwarded",
				},
			}
			usage, newAPIError := tt.handle(c, info, &http.Response{Body: io.NopCloser(strings.NewReader(payload))})
			require.Nil(t, newAPIError)
			require.NotNil(t, usage)
			assert.Equal(t, "gemini-forwarded", info.ForwardedModelName)
			assert.Equal(t, "gemini-upstream-version", info.ActualResponseModel)
			assert.Equal(t, "gemini-public", info.OriginModelName)
			assert.Equal(t, "gemini-forwarded", info.UpstreamModelName)
			assert.Equal(t, 2, usage.PromptTokens)
			assert.Equal(t, 3, usage.CompletionTokens)
			assert.Equal(t, 5, usage.TotalTokens)
			assert.Equal(t, http.StatusOK, recorder.Code)
			if tt.format == types.RelayFormatGemini {
				assert.Equal(t, payload, recorder.Body.String())
			} else {
				assert.Contains(t, recorder.Body.String(), `"hello"`)
			}
		})
	}
}

func TestGeminiNativeStreamCapturesLateModelVersionWithoutChangingChunks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	const first = `{"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]}}]}`
	const final = `{"modelVersion":"gemini-upstream-version","candidates":[{"content":{"role":"model","parts":[]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":3,"totalTokenCount":5}}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-public:streamGenerateContent", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatGemini,
		OriginModelName: "gemini-public",
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "gemini-forwarded",
		},
	}
	body := "data: " + first + "\n\ndata: " + final + "\n\n"
	usage, newAPIError := GeminiTextGenerationStreamHandler(c, info, &http.Response{Body: io.NopCloser(strings.NewReader(body))})
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, "gemini-forwarded", info.ForwardedModelName)
	assert.Equal(t, "gemini-upstream-version", info.ActualResponseModel)
	assert.Equal(t, 2, usage.PromptTokens)
	assert.Equal(t, 3, usage.CompletionTokens)
	assert.Equal(t, 5, usage.TotalTokens)
	assert.Contains(t, recorder.Body.String(), "data: "+first)
	assert.Contains(t, recorder.Body.String(), "data: "+final)
}
