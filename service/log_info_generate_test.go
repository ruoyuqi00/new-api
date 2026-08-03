package service

import (
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
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
