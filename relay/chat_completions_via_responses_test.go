package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsResponsesEventStreamContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{name: "plain", contentType: "text/event-stream", want: true},
		{name: "mixed case with charset", contentType: "Text/Event-Stream; charset=utf-8", want: true},
		{name: "json", contentType: "application/json", want: false},
		{name: "empty", contentType: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isResponsesEventStreamContentType(tt.contentType))
		})
	}
}

func TestChatCompletionsViaResponsesAppliesChannelOutputLimitPolicy(t *testing.T) {
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		ignoreLimit bool
		wantLimit   bool
	}{
		{name: "preserves client limit by default", wantLimit: true},
		{name: "removes client limit when enabled", ignoreLimit: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstreamBody := make(chan map[string]any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				require.NoError(t, common.DecodeJson(r.Body, &body))
				upstreamBody <- body
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"message":"fixture complete","type":"invalid_request_error"}}`))
			}))
			t.Cleanup(server.Close)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			maxCompletionTokens := uint(4096)
			request := &dto.GeneralOpenAIRequest{
				Model: "gpt-5.1",
				Messages: []dto.Message{{
					Role:    "user",
					Content: "hello",
				}},
				MaxCompletionTokens: &maxCompletionTokens,
			}
			info := &relaycommon.RelayInfo{
				OriginModelName: "gpt-5.1",
				RelayMode:       relayconstant.RelayModeChatCompletions,
				RelayFormat:     types.RelayFormatOpenAI,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:       constant.ChannelTypeOpenAI,
					ApiType:           constant.APITypeOpenAI,
					ChannelBaseUrl:    server.URL,
					ApiKey:            "test-key",
					UpstreamModelName: "gpt-5.1",
					ChannelOtherSettings: dto.ChannelOtherSettings{
						IgnoreClientMaxOutputTokens: tt.ignoreLimit,
					},
				},
			}
			adaptor := &openaichannel.Adaptor{}
			adaptor.Init(info)

			_, relayErr := chatCompletionsViaResponses(c, info, adaptor, request)

			require.NotNil(t, relayErr)
			body := <-upstreamBody
			_, hasLimit := body["max_output_tokens"]
			assert.Equal(t, tt.wantLimit, hasLimit)
		})
	}
}
