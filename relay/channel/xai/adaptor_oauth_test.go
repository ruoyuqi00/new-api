package xai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupRequestHeaderUsesOAuthAccessToken(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	headers := http.Header{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ApiKey: `{"access_token":"oauth-access","refresh_token":"oauth-refresh"}`,
	}}

	err := (&Adaptor{}).SetupRequestHeader(context, &headers, info)
	require.NoError(t, err)
	assert.Equal(t, "Bearer oauth-access", headers.Get("Authorization"))
}

func TestSetupRequestHeaderKeepsAPIKey(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	headers := http.Header{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "xai-key"}}

	err := (&Adaptor{}).SetupRequestHeader(context, &headers, info)
	require.NoError(t, err)
	assert.Equal(t, "Bearer xai-key", headers.Get("Authorization"))
}
