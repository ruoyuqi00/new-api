package service

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestValidGPTTextUsageRejectsMalformedOrAmplifiedUsage(t *testing.T) {
	tests := []struct {
		name  string
		usage *dto.Usage
		valid bool
	}{
		{
			name: "valid upstream usage",
			usage: &dto.Usage{
				PromptTokens:        1200,
				CompletionTokens:    25,
				TotalTokens:         1225,
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 1024, CacheWriteTokens: 128},
			},
			valid: true,
		},
		{name: "nil usage", usage: nil, valid: false},
		{name: "empty usage", usage: &dto.Usage{}, valid: false},
		{name: "negative token", usage: &dto.Usage{PromptTokens: -1, CompletionTokens: 1, TotalTokens: 0}, valid: false},
		{
			name:  "inconsistent total",
			usage: &dto.Usage{PromptTokens: 1200, CompletionTokens: 25, TotalTokens: 1200},
			valid: false,
		},
		{
			name:  "cache exceeds input",
			usage: &dto.Usage{PromptTokens: 1200, CompletionTokens: 25, TotalTokens: 1225, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 1201}},
			valid: false,
		},
		{
			name:  "geometric amplification",
			usage: &dto.Usage{PromptTokens: 10_000_001, CompletionTokens: 1, TotalTokens: 10_000_002},
			valid: false,
		},
		{
			name:  "max int amplification",
			usage: &dto.Usage{PromptTokens: math.MaxInt, CompletionTokens: 1, TotalTokens: math.MaxInt},
			valid: false,
		},
		{
			name: "negative nested cache",
			usage: &dto.Usage{
				PromptTokens: 1200, CompletionTokens: 25, TotalTokens: 1225,
				InputTokensDetails: &dto.InputTokenDetails{CachedTokens: -1},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.valid, ValidGPTTextUsage(tt.usage))
		})
	}
}

func TestGPTTextSettlementRecognizesAllSupportedTextEndpoints(t *testing.T) {
	for _, path := range []string{
		"/v1/chat/completions",
		"/v1/completions",
		"/v1/responses",
		"/v1/responses/compact",
	} {
		t.Run(path, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, path, nil)
			require.True(t, isGPTTextSettlementRequest(ctx, &relaycommon.RelayInfo{RequestURLPath: path}))
		})
	}
}
