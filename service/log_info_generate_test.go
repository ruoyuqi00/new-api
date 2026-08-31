package service

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGenerateTextOtherInfoKeepsForwardedModelSnapshot(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	now := time.Now()
	info := &relaycommon.RelayInfo{
		StartTime:           now,
		FirstResponseTime:   now,
		ForwardedModelName:  "forwarded-model",
		ActualResponseModel: "response-model",
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "response-model",
		},
	}

	other := GenerateTextOtherInfo(c, info, 1, 1, 1, 0, 0, 0, 1)

	assert.Equal(t, true, other["is_model_mapped"])
	assert.Equal(t, "forwarded-model", other["upstream_model_name"])
}

func TestGenerateTextOtherInfoDoesNotExposeRawStreamErrors(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	status := relaycommon.NewStreamStatus()
	status.SetEndReason(relaycommon.StreamEndReasonClientGone, errors.New("POST https://private-upstream.example/v1/responses: Bearer secret-token"))
	status.RecordError("Authorization: Bearer another-secret-token")
	info := &relaycommon.RelayInfo{
		StartTime:         time.Now(),
		FirstResponseTime: time.Now(),
		IsStream:          true,
		StreamStatus:      status,
		ChannelMeta:       &relaycommon.ChannelMeta{},
	}

	other := GenerateTextOtherInfo(c, info, 1, 1, 1, 0, 0, 0, 1)
	streamInfo, ok := other["stream_status"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "error", streamInfo["status"])
	assert.Equal(t, string(relaycommon.StreamEndReasonClientGone), streamInfo["end_reason"])
	assert.Equal(t, 1, streamInfo["error_count"])
	assert.NotContains(t, streamInfo, "end_error")
	assert.NotContains(t, streamInfo, "errors")
}

func TestGenerateTextOtherInfoIncludesImageResolutionAuditWithoutUpstreamSecrets(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/images/generations", nil)
	now := time.Now()
	info := &relaycommon.RelayInfo{
		StartTime:         now,
		FirstResponseTime: now,
		ChannelMeta:       &relaycommon.ChannelMeta{},
		PriceData: types.PriceData{
			ImageResolutionPricingModel:  "gpt-image-2",
			ImageResolutionRequestedSize: "650x1024",
			ImageResolutionTier:          "1k",
			ImageResolutionUnitPrice:     0.01,
			ImageResolutionImageCount:    2,
		},
	}

	other := GenerateTextOtherInfo(c, info, 0, 0.3, 0, 0, 0, 0.01, -1)
	assert.Equal(t, "gpt-image-2", other["image_pricing_model"])
	assert.Equal(t, "650x1024", other["image_requested_size"])
	assert.Equal(t, "1k", other["image_resolution_tier"])
	assert.Equal(t, 0.01, other["image_unit_price"])
	assert.Equal(t, 2, other["image_count"])
	assert.NotContains(t, other, "upstream_url")
	assert.NotContains(t, other, "api_key")
}
