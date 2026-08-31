package gemini

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestPreservesTierAndAspectRatio(t *testing.T) {
	ratio := "3:2"
	converted, err := (&Adaptor{}).ConvertImageRequest(
		gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()),
		&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "imagen-4.0-generate-001"}},
		dto.ImageRequest{Model: "gpt-image-2", Prompt: "a landscape", Size: "2k", AspectRatio: &ratio},
	)
	require.NoError(t, err)
	payload, err := common.Marshal(converted)
	require.NoError(t, err)
	require.JSONEq(t, `{"instances":[{"prompt":"a landscape"}],"parameters":{"sampleCount":1,"aspectRatio":"3:2","personGeneration":"allow_adult","imageSize":"2K"}}`, string(payload))
}

func TestConvertImageRequestDerivesRatioFromExactDimensions(t *testing.T) {
	converted, err := (&Adaptor{}).ConvertImageRequest(
		gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()),
		&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "imagen-4.0-generate-001"}},
		dto.ImageRequest{Model: "gpt-image-2", Prompt: "a portrait", Size: "650x1024"},
	)
	require.NoError(t, err)
	payload, err := common.Marshal(converted)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"aspectRatio":"325:512"`)
	require.Contains(t, string(payload), `"imageSize":"1K"`)
}

func TestConvertImageRequestRejectsUnsupportedFourK(t *testing.T) {
	_, err := (&Adaptor{}).ConvertImageRequest(
		gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()),
		&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "imagen-4.0-generate-001"}},
		dto.ImageRequest{Model: "gpt-image-2", Prompt: "large", Size: "4k"},
	)
	require.Error(t, err)
}
