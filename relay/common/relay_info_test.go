package common

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInitChannelMetaResetsResponseModelAuditForRetry(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &RelayInfo{
		ForwardedModelName:  "failed-forwarded-model",
		ActualResponseModel: "failed-response-model",
	}

	info.InitChannelMeta(c)

	require.Empty(t, info.ForwardedModelName)
	require.Empty(t, info.ActualResponseModel)
}

func TestInitChannelMetaResetsStreamOutcomeForRetry(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &RelayInfo{
		ChannelAffinityResponseID:         "resp_failed_attempt",
		ChannelAffinityResponseIDObserved: true,
		StreamStatus:                      NewStreamStatus(),
		StreamTerminalMarkersRequired:     true,
		StreamTerminalSuccess:             true,
		StreamTerminalUsageSeen:           true,
	}

	info.InitChannelMeta(c)

	require.Nil(t, info.StreamStatus)
	require.Empty(t, info.ChannelAffinityResponseID)
	require.False(t, info.ChannelAffinityResponseIDObserved)
	require.False(t, info.StreamTerminalMarkersRequired)
	require.False(t, info.StreamTerminalSuccess)
	require.False(t, info.StreamTerminalUsageSeen)
}

func TestRelayInfoGetFinalRequestRelayFormatPrefersExplicitFinal(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		RequestConversionChain:  []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToConversionChain(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToRelayFormat(t *testing.T) {
	info := &RelayInfo{
		RelayFormat: types.RelayFormatGemini,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatGemini), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatNilReceiver(t *testing.T) {
	var info *RelayInfo
	require.Equal(t, types.RelayFormat(""), info.GetFinalRequestRelayFormat())
}
