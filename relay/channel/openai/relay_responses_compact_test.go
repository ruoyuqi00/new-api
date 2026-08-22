package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOaiResponsesCompactionHandlerSanitizesAmplifiedUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAIResponsesCompaction,
		RequestURLPath:  "/v1/responses/compact",
		OriginModelName: "gpt-test",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	info.SetEstimatePromptTokens(400)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"cmp-1","object":"response.compaction","output":[],"usage":{"input_tokens":10000001,"output_tokens":1,"total_tokens":10000002}}`)),
	}

	usage, relayErr := OaiResponsesCompactionHandler(c, info, resp)

	require.Nil(t, relayErr)
	require.Equal(t, "estimated", usage.UsageSource)
	require.Equal(t, 400, usage.PromptTokens)
	require.NotContains(t, recorder.Body.String(), "10000001")
	require.True(t, info.PreservePreConsumedQuota)
}
