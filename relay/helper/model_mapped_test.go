package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDomesticCallAliasesPreservePublicModelAndMapUpstreamModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		publicName   string
		upstreamName string
	}{
		{publicName: "deepseek-v4-flash-0731-call", upstreamName: "deepseek-v4-flash-0731"},
		{publicName: "deepseek-v4-pro-0813-call", upstreamName: "deepseek-v4-pro-0813"},
		{publicName: "glm-5.2-call", upstreamName: "glm-5.2"},
		{publicName: "kimi-k3-call", upstreamName: "kimi-k3"},
		{publicName: "MiniMax-M2.7-call", upstreamName: "MiniMax-M2.7"},
		{publicName: "qwen3.8-max-call", upstreamName: "qwen3.8-max"},
	}

	for _, tc := range cases {
		t.Run(tc.publicName, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Set("model_mapping", `{"`+tc.publicName+`":"`+tc.upstreamName+`"}`)
			request := &dto.GeneralOpenAIRequest{Model: tc.publicName}
			info := &relaycommon.RelayInfo{OriginModelName: tc.publicName}

			require.NoError(t, ModelMappedHelper(ctx, info, request))
			require.Equal(t, tc.publicName, info.OriginModelName)
			require.Equal(t, tc.upstreamName, info.UpstreamModelName)
			require.True(t, info.IsModelMapped)
			require.Equal(t, tc.upstreamName, request.Model)
		})
	}
}
