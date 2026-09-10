package xai

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestMapsDimensionsToSupportedXAIParameters(t *testing.T) {
	explicitRatio := "9:16"
	tests := []struct {
		name           string
		model          string
		size           string
		aspectRatio    *string
		wantRatio      string
		wantResolution string
	}{
		{name: "custom portrait", size: "650x1024", wantRatio: "2:3", wantResolution: "1k"},
		{name: "legacy custom landscape", size: "2000x800", wantRatio: "20:9", wantResolution: "2k"},
		{name: "image 2 custom landscape", size: "2000x800", model: "grok-imagine-image-2.0", wantRatio: "5:2", wantResolution: "2k"},
		{name: "explicit ratio wins", size: "650x1024", aspectRatio: &explicitRatio, wantRatio: "9:16", wantResolution: "1k"},
		{name: "auto ratio derives from size", size: "650x1024", aspectRatio: stringPointer("auto"), wantRatio: "2:3", wantResolution: "1k"},
		{name: "custom explicit ratio is normalized", size: "1k", aspectRatio: stringPointer("13:7"), wantRatio: "16:9", wantResolution: "1k"},
		{name: "tier only", size: "2k", wantResolution: "2k"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelName := tt.model
			if modelName == "" {
				modelName = "grok-imagine-image"
			}
			converted, err := (&Adaptor{}).ConvertImageRequest(
				&gin.Context{},
				&relaycommon.RelayInfo{},
				dto.ImageRequest{
					Model:       modelName,
					Prompt:      "portrait",
					Size:        tt.size,
					AspectRatio: tt.aspectRatio,
				},
			)
			require.NoError(t, err)

			request, ok := converted.(ImageRequest)
			require.True(t, ok)
			assert.Equal(t, tt.wantRatio, request.AspectRatio)
			assert.Equal(t, tt.wantResolution, request.Resolution)
		})
	}
}

func TestConvertImageRequestValidatesResolutionBoundaries(t *testing.T) {
	tests := []struct {
		size           string
		wantResolution string
		wantError      bool
	}{
		{size: "1024x1024", wantResolution: "1k"},
		{size: "1025x1024", wantResolution: "2k"},
		{size: "2048x2048", wantResolution: "2k"},
		{size: "2049x1024", wantError: true},
		{size: "4k", wantError: true},
		{size: "wide", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			converted, err := (&Adaptor{}).ConvertImageRequest(&gin.Context{}, &relaycommon.RelayInfo{}, dto.ImageRequest{Model: "grok-imagine-image", Prompt: "test", Size: tt.size})
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantResolution, converted.(ImageRequest).Resolution)
		})
	}
}

func stringPointer(value string) *string {
	return &value
}
