package sora

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func buildJSONRequestBody(t *testing.T, body string, upstreamModel string) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", gin.MIMEJSON)

	requestBody, err := (&TaskAdaptor{}).BuildRequestBody(c, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: upstreamModel},
	})
	require.NoError(t, err)
	data, err := io.ReadAll(requestBody)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, common.Unmarshal(data, &result))
	return result
}

func TestBuildRequestBodyNormalizesAutoResolutionForVeo31(t *testing.T) {
	body := buildJSONRequestBody(t,
		`{"model":"veo-3-1-fast","prompt":"test","resolution":" Auto "}`,
		"veo-3-1-fast",
	)

	require.Equal(t, "veo-3-1-fast", body["model"])
	require.Equal(t, "720p", body["resolution"])
}

func TestBuildRequestBodyLeavesAutoResolutionForOtherSoraModels(t *testing.T) {
	body := buildJSONRequestBody(t,
		`{"model":"sora-2","prompt":"test","resolution":"Auto"}`,
		"sora-2",
	)

	require.Equal(t, "sora-2", body["model"])
	require.Equal(t, "Auto", body["resolution"])
}
