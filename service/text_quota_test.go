package service

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestShouldObserveConfirmedChannelAffinityUsage(t *testing.T) {
	tests := []struct {
		name       string
		canceled   bool
		relayInfo  *relaycommon.RelayInfo
		usage      *dto.Usage
		wantResult bool
	}{
		{
			name:       "non-stream success",
			relayInfo:  &relaycommon.RelayInfo{},
			usage:      &dto.Usage{},
			wantResult: true,
		},
		{
			name:      "client canceled",
			canceled:  true,
			relayInfo: &relaycommon.RelayInfo{},
			usage:     &dto.Usage{},
		},
		{
			name: "stream completed",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:     true,
				StreamStatus: streamStatusForAffinityUsageTest(relaycommon.StreamEndReasonDone, false),
			},
			usage:      &dto.Usage{},
			wantResult: true,
		},
		{
			name: "stream without terminal status",
			relayInfo: &relaycommon.RelayInfo{
				IsStream: true,
			},
			usage: &dto.Usage{},
		},
		{
			name: "responses stream missing terminal usage",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:                      true,
				RelayFormat:                   types.RelayFormatOpenAIResponses,
				StreamStatus:                  streamStatusForAffinityUsageTest(relaycommon.StreamEndReasonDone, false),
				StreamTerminalMarkersRequired: true,
				StreamTerminalSuccess:         true,
			},
			usage: &dto.Usage{PromptTokens: 100},
		},
		{
			name: "responses stream exact terminal usage",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:                      true,
				RelayFormat:                   types.RelayFormatOpenAIResponses,
				StreamStatus:                  streamStatusForAffinityUsageTest(relaycommon.StreamEndReasonDone, false),
				StreamTerminalMarkersRequired: true,
				StreamTerminalSuccess:         true,
				StreamTerminalUsageSeen:       true,
			},
			usage:      &dto.Usage{PromptTokens: 100},
			wantResult: true,
		},
		{
			name: "responses adapter authoritative usage",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:     true,
				RelayFormat:  types.RelayFormatOpenAIResponses,
				StreamStatus: streamStatusForAffinityUsageTest(relaycommon.StreamEndReasonDone, false),
			},
			usage:      &dto.Usage{PromptTokens: 100},
			wantResult: true,
		},
		{
			name: "stream client gone",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:     true,
				StreamStatus: streamStatusForAffinityUsageTest(relaycommon.StreamEndReasonClientGone, false),
			},
			usage: &dto.Usage{},
		},
		{
			name: "stream eof with incomplete error",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:     true,
				StreamStatus: streamStatusForAffinityUsageTest(relaycommon.StreamEndReasonEOF, true),
			},
			usage: &dto.Usage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			requestCtx, cancel := context.WithCancel(context.Background())
			if tt.canceled {
				cancel()
			} else {
				t.Cleanup(cancel)
			}
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestCtx)

			require.Equal(t, tt.wantResult, shouldObserveConfirmedChannelAffinityUsage(ctx, tt.relayInfo, tt.usage))
		})
	}
}

func streamStatusForAffinityUsageTest(reason relaycommon.StreamEndReason, withError bool) *relaycommon.StreamStatus {
	status := relaycommon.NewStreamStatus()
	status.SetEndReason(reason, nil)
	if withError {
		status.RecordError("upstream stream ended before completion")
	}
	return status
}

func TestCalculateTextQuotaSummaryUnifiedForClaudeSemantic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	usage := &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         100,
			CachedCreationTokens: 50,
		},
		ClaudeCacheCreation5mTokens: 10,
		ClaudeCacheCreation1hTokens: 20,
	}

	priceData := types.PriceData{
		ModelRatio:           1,
		CompletionRatio:      2,
		CacheRatio:           0.1,
		CacheCreationRatio:   1.25,
		CacheCreation5mRatio: 1.25,
		CacheCreation1hRatio: 2,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio: 1,
		},
	}

	chatRelayInfo := &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		FinalRequestRelayFormat: types.RelayFormatClaude,
		OriginModelName:         "claude-3-7-sonnet",
		PriceData:               priceData,
		StartTime:               time.Now(),
	}
	messageRelayInfo := &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatClaude,
		FinalRequestRelayFormat: types.RelayFormatClaude,
		OriginModelName:         "claude-3-7-sonnet",
		PriceData:               priceData,
		StartTime:               time.Now(),
	}

	chatSummary := calculateTextQuotaSummary(ctx, chatRelayInfo, usage)
	messageSummary := calculateTextQuotaSummary(ctx, messageRelayInfo, usage)

	require.Equal(t, messageSummary.Quota, chatSummary.Quota)
	require.Equal(t, messageSummary.CacheCreationTokens5m, chatSummary.CacheCreationTokens5m)
	require.Equal(t, messageSummary.CacheCreationTokens1h, chatSummary.CacheCreationTokens1h)
	require.True(t, chatSummary.IsClaudeUsageSemantic)
	require.Equal(t, 1488, chatSummary.Quota)
}

func TestCalculateTextQuotaSummaryUsesSplitClaudeCacheCreationRatios(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		FinalRequestRelayFormat: types.RelayFormatClaude,
		OriginModelName:         "claude-3-7-sonnet",
		PriceData: types.PriceData{
			ModelRatio:           1,
			CompletionRatio:      1,
			CacheRatio:           0,
			CacheCreationRatio:   1,
			CacheCreation5mRatio: 2,
			CacheCreation1hRatio: 3,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 0,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedCreationTokens: 10,
		},
		ClaudeCacheCreation5mTokens: 2,
		ClaudeCacheCreation1hTokens: 3,
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// 100 + remaining(5)*1 + 2*2 + 3*3 = 118
	require.Equal(t, 118, summary.Quota)
}

func TestCalculateTextQuotaSummaryUsesAnthropicUsageSemanticFromUpstreamUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "claude-3-7-sonnet",
		PriceData: types.PriceData{
			ModelRatio:           1,
			CompletionRatio:      2,
			CacheRatio:           0.1,
			CacheCreationRatio:   1.25,
			CacheCreation5mRatio: 1.25,
			CacheCreation1hRatio: 2,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		UsageSemantic:    "anthropic",
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         100,
			CachedCreationTokens: 50,
		},
		ClaudeCacheCreation5mTokens: 10,
		ClaudeCacheCreation1hTokens: 20,
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	require.True(t, summary.IsClaudeUsageSemantic)
	require.Equal(t, "anthropic", summary.UsageSemantic)
	require.Equal(t, 1488, summary.Quota)
}

func TestCacheWriteTokensTotal(t *testing.T) {
	t.Run("split cache creation", func(t *testing.T) {
		summary := textQuotaSummary{
			CacheCreationTokens:   50,
			CacheCreationTokens5m: 10,
			CacheCreationTokens1h: 20,
		}
		require.Equal(t, 50, cacheWriteTokensTotal(summary))
	})

	t.Run("legacy cache creation", func(t *testing.T) {
		summary := textQuotaSummary{CacheCreationTokens: 50}
		require.Equal(t, 50, cacheWriteTokensTotal(summary))
	})

	t.Run("split cache creation without aggregate remainder", func(t *testing.T) {
		summary := textQuotaSummary{
			CacheCreationTokens5m: 10,
			CacheCreationTokens1h: 20,
		}
		require.Equal(t, 30, cacheWriteTokensTotal(summary))
	})
}

func TestCalculateTextQuotaSummaryHandlesLegacyClaudeDerivedOpenAIUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "claude-3-7-sonnet",
		PriceData: types.PriceData{
			ModelRatio:           1,
			CompletionRatio:      5,
			CacheRatio:           0.1,
			CacheCreationRatio:   1.25,
			CacheCreation5mRatio: 1.25,
			CacheCreation1hRatio: 2,
			GroupRatioInfo:       types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     62,
		CompletionTokens: 95,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 3544,
		},
		ClaudeCacheCreation5mTokens: 586,
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// 62 + 3544*0.1 + 586*1.25 + 95*5 = 1624.9 => 1624
	require.Equal(t, 1624, summary.Quota)
}

func TestCalculateTextQuotaSummarySeparatesOpenRouterCacheReadFromPromptBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "openai/gpt-4.1",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenRouter,
		},
		PriceData: types.PriceData{
			ModelRatio:         1,
			CompletionRatio:    1,
			CacheRatio:         0.1,
			CacheCreationRatio: 1.25,
			GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     2604,
		CompletionTokens: 383,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 2432,
		},
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// OpenRouter OpenAI-format display keeps prompt_tokens as total input,
	// but billing still separates normal input from cache read tokens.
	// quota = (2604 - 2432) + 2432*0.1 + 383 = 798.2 => 798
	require.Equal(t, 2604, summary.PromptTokens)
	require.Equal(t, 798, summary.Quota)
}

func TestCalculateTextQuotaSummarySeparatesOpenRouterCacheCreationFromPromptBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "openai/gpt-4.1",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenRouter,
		},
		PriceData: types.PriceData{
			ModelRatio:         1,
			CompletionRatio:    1,
			CacheCreationRatio: 1.25,
			GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     2604,
		CompletionTokens: 383,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedCreationTokens: 100,
		},
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// prompt_tokens is still logged as total input, but cache creation is billed separately.
	// quota = (2604 - 100) + 100*1.25 + 383 = 3012
	require.Equal(t, 2604, summary.PromptTokens)
	require.Equal(t, 3012, summary.Quota)
}

func TestCalculateTextQuotaSummaryKeepsPrePRClaudeOpenRouterBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		FinalRequestRelayFormat: types.RelayFormatClaude,
		OriginModelName:         "anthropic/claude-3.7-sonnet",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenRouter,
		},
		PriceData: types.PriceData{
			ModelRatio:         1,
			CompletionRatio:    1,
			CacheRatio:         0.1,
			CacheCreationRatio: 1.25,
			GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     2604,
		CompletionTokens: 383,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 2432,
		},
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// Pre-PR PostClaudeConsumeQuota behavior for OpenRouter:
	// prompt = 2604 - 2432 = 172
	// quota = 172 + 2432*0.1 + 383 = 798.2 => 798
	require.True(t, summary.IsClaudeUsageSemantic)
	require.Equal(t, 172, summary.PromptTokens)
	require.Equal(t, 798, summary.Quota)
}

func TestComposeTieredTextQuotaKeepsToolCallSurcharges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("image_generation_call", true)
	ctx.Set("image_generation_call_quality", "low")
	ctx.Set("image_generation_call_size", "1024x1024")

	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "o1",
		PriceData: types.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1},
		},
		ResponsesUsageInfo: &relaycommon.ResponsesUsageInfo{
			BuiltInTools: map[string]*relaycommon.BuildInToolInfo{
				dto.BuildInToolWebSearchPreview: &relaycommon.BuildInToolInfo{
					CallCount: 1,
				},
				dto.BuildInToolFileSearch: &relaycommon.BuildInToolInfo{
					CallCount: 2,
				},
			},
		},
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:               "tiered_expr",
			GroupRatio:                1,
			EstimatedQuotaBeforeGroup: 1000,
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
	quota := composeTieredTextQuota(relayInfo, summary, 1000, &billingexpr.TieredResult{
		ActualQuotaBeforeGroup: 1000,
		ActualQuotaAfterGroup:  1000,
	})

	require.Equal(t, int64(13000), summary.ToolCallSurchargeQuota.Round(0).IntPart())
	require.Equal(t, 14000, quota)
}

func TestCalculateTextQuotaSummaryImageGenerationOnlyBillingForGPTImage2(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("image_generation_call", true)
	ctx.Set("image_generation_call_quality", "high")
	ctx.Set("image_generation_call_size", "1024x1024")

	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		PriceData: types.PriceData{
			ModelPrice:      0.05,
			UsePrice:        true,
			GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 0.5},
			CompletionRatio: 1,
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     2309,
		CompletionTokens: 48,
		TotalTokens:      2357,
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	require.True(t, summary.ImageGenerationOnlyBilling)
	require.Equal(t, 0.0, summary.ModelPrice)
	require.Equal(t, 0.0, summary.CompletionRatio)
	require.Equal(t, 12500, summary.Quota)
}

func TestImageGenerationOnlyBillingRecognizesGPTImage2Aliases(t *testing.T) {
	require.True(t, isImageGenerationOnlyBillingModel("gpt-image-2-1k"))
	require.True(t, isImageGenerationOnlyBillingModel("openai/gpt-image-2-4k"))
	require.False(t, isImageGenerationOnlyBillingModel("gpt-image-1.5"))
}

func TestCalculateTextQuotaSummaryBillsOpenAICacheWriteTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "gpt-5.1",
		PriceData: types.PriceData{
			ModelRatio:         1,
			CompletionRatio:    2,
			CacheRatio:         0.1,
			CacheCreationRatio: 1.25,
			GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	t.Run("uncached remainder stays positive", func(t *testing.T) {
		usage := &dto.Usage{
			PromptTokens:     1473,
			CompletionTokens: 19,
			PromptTokensDetails: dto.InputTokenDetails{
				CacheWriteTokens: 1470,
			},
		}

		summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
		require.Equal(t, 1470, summary.CacheCreationTokens)
		require.Equal(t, 1879, summary.Quota)
	})

	t.Run("overlapping prefixes clamp uncached remainder", func(t *testing.T) {
		usage := &dto.Usage{
			PromptTokens:     3619,
			CompletionTokens: 36,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens:     2921,
				CacheWriteTokens: 3616,
			},
		}

		summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
		require.Equal(t, 3616, summary.CacheCreationTokens)
		require.Equal(t, 4884, summary.Quota)
	})
}

func TestComposeTieredTextQuotaFallbackKeepsToolCallSurcharges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("claude_web_search_requests", 2)

	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-7-sonnet",
		PriceData: types.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1.25},
		},
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:               "tiered_expr",
			GroupRatio:                1.25,
			EstimatedQuotaBeforeGroup: 1000,
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
	quota := composeTieredTextQuota(relayInfo, summary, 1250, nil)

	require.Equal(t, int64(12500), summary.ToolCallSurchargeQuota.Round(0).IntPart())
	require.Equal(t, 13750, quota)
}

func TestComposeTieredTextQuotaErrorFallbackUsesPreConsumedQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("claude_web_search_requests", 2)

	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-7-sonnet",
		PriceData: types.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1.25},
		},
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:               "tiered_expr",
			GroupRatio:                1.25,
			EstimatedQuotaBeforeGroup: 1000,
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	// tieredResult=nil simulates a settlement error where TryTieredSettle
	// falls back to FinalPreConsumedQuota (2000), which differs from
	// EstimatedQuotaBeforeGroup * GroupRatio (1250).
	preConsumedFallback := 2000
	quota := composeTieredTextQuota(relayInfo, summary, preConsumedFallback, nil)

	require.Equal(t, int64(12500), summary.ToolCallSurchargeQuota.Round(0).IntPart())
	require.Equal(t, 14500, quota)
}

func TestComposeTieredTextQuotaSaturatesFallbackTotal(t *testing.T) {
	summary := textQuotaSummary{
		ToolCallSurchargeQuota: decimal.NewFromInt(100),
	}

	quota := composeTieredTextQuota(&relaycommon.RelayInfo{}, summary, math.MaxInt32-50, nil)

	require.Equal(t, math.MaxInt32, quota)
}

func TestComposeTieredTextQuotaSaturatesTieredResultTotal(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			GroupRatio: 1,
		},
	}
	summary := textQuotaSummary{
		ToolCallSurchargeQuota: decimal.NewFromInt(100),
	}

	quota := composeTieredTextQuota(relayInfo, summary, 0, &billingexpr.TieredResult{
		ActualQuotaBeforeGroup: float64(math.MaxInt32 - 50),
	})

	require.Equal(t, math.MaxInt32, quota)
}

func TestApplyPreConsumedQuotaFloor(t *testing.T) {
	tests := []struct {
		name          string
		relayInfo     *relaycommon.RelayInfo
		calculated    int
		wantQuota     int
		wantPreserved bool
	}{
		{
			name: "incomplete accepted stream keeps pre-consumed quota",
			relayInfo: &relaycommon.RelayInfo{
				PreservePreConsumedQuota: true,
				FinalPreConsumedQuota:    1000,
			},
			calculated:    250,
			wantQuota:     1000,
			wantPreserved: true,
		},
		{
			name: "higher actual charge remains unchanged",
			relayInfo: &relaycommon.RelayInfo{
				PreservePreConsumedQuota: true,
				FinalPreConsumedQuota:    1000,
			},
			calculated: 1250,
			wantQuota:  1250,
		},
		{
			name: "unconfirmed terminal usage cannot raise charge above frozen reservation",
			relayInfo: &relaycommon.RelayInfo{
				PreservePreConsumedQuota:      true,
				FinalPreConsumedQuota:         1000,
				StreamTerminalMarkersRequired: true,
			},
			calculated: 2500,
			wantQuota:  2500,
		},
		{
			name: "trusted billing uses frozen price estimate when no quota was reserved",
			relayInfo: &relaycommon.RelayInfo{
				PreservePreConsumedQuota:      true,
				StreamTerminalMarkersRequired: true,
				PriceData:                     types.PriceData{QuotaToPreConsume: 1400},
			},
			calculated: 2500,
			wantQuota:  2500,
		},
		{
			name: "current tiered estimate is the frozen reservation",
			relayInfo: &relaycommon.RelayInfo{
				PreservePreConsumedQuota:      true,
				StreamTerminalMarkersRequired: true,
				PriceData:                     types.PriceData{QuotaToPreConsume: 1400},
				TieredBillingSnapshot: &billingexpr.BillingSnapshot{
					BillingMode:              "tiered_expr",
					EstimatedQuotaAfterGroup: 900,
				},
			},
			calculated: 2500,
			wantQuota:  2500,
		},
		{
			name: "ordinary stream can settle below pre-consume",
			relayInfo: &relaycommon.RelayInfo{
				FinalPreConsumedQuota: 1000,
			},
			calculated: 250,
			wantQuota:  250,
		},
		{
			name:       "nil relay info",
			calculated: 250,
			wantQuota:  250,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quota, preserved := applyPreConsumedQuotaFloor(tt.relayInfo, tt.calculated)

			require.Equal(t, tt.wantQuota, quota)
			require.Equal(t, tt.wantPreserved, preserved)
		})
	}
}

func TestAmbiguousTextBillingSettlesFrozenReservation(t *testing.T) {
	truncate(t)
	seedUser(t, 101, 0)
	seedChannel(t, 201)
	tests := []struct {
		name       string
		info       *relaycommon.RelayInfo
		want       int
		wantPrompt int
	}{
		{
			name: "legacy estimate",
			info: &relaycommon.RelayInfo{
				UserId: 101, TokenId: 301, UsingGroup: "default", OriginModelName: "gpt-test",
				StartTime:   time.Now(),
				ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 201},
				PriceData:   types.PriceData{ModelRatio: 1, CompletionRatio: 1, QuotaToPreConsume: 1250, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}},
			},
			want:       3200,
			wantPrompt: 3200,
		},
		{
			name: "selected tier estimate",
			info: &relaycommon.RelayInfo{
				UserId: 101, TokenId: 301, UsingGroup: "default", OriginModelName: "gpt-test",
				StartTime:   time.Now(),
				ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 201},
				PriceData:   types.PriceData{QuotaToPreConsume: 1250},
				TieredBillingSnapshot: &billingexpr.BillingSnapshot{
					BillingMode:              "tiered_expr",
					EstimatedQuotaAfterGroup: 875,
					EstimatedTier:            "selected-at-reservation",
				},
			},
			want:       875,
			wantPrompt: 3200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			billing := &recordingTaskBillingSettler{}
			tt.info.Billing = billing
			tt.info.SetEstimatePromptTokens(tt.wantPrompt)
			attempt := tt.info.BeginUpstreamRequestAttempt()
			attempt.MarkRequestWritten()
			attempt.MarkAmbiguousIfPotentiallySent()

			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			err := SettleAmbiguousTextBilling(ctx, tt.info)

			require.NoError(t, err)
			require.Equal(t, []int{tt.want}, billing.settled)
			require.True(t, tt.info.PreservePreConsumedQuota)
			var logs []model.Log
			require.NoError(t, model.LOG_DB.Where("user_id = ?", tt.info.UserId).Find(&logs).Error)
			require.Len(t, logs, 1)
			require.Equal(t, tt.want, logs[0].Quota)
			require.Equal(t, tt.wantPrompt, logs[0].PromptTokens)
			require.Zero(t, logs[0].CompletionTokens)
			require.Contains(t, logs[0].Other, "usage_unconfirmed")
			require.Contains(t, logs[0].Other, `"usage_source":"estimated"`)
			if tt.info.TieredBillingSnapshot != nil {
				require.Contains(t, logs[0].Other, `"billing_mode":"tiered_expr"`)
				require.Contains(t, logs[0].Other, `"estimated_tier":"selected-at-reservation"`)
				require.Contains(t, logs[0].Other, `"settled_from_reservation":true`)
			}

			var user model.User
			require.NoError(t, model.DB.First(&user, tt.info.UserId).Error)
			require.Equal(t, tt.want, user.UsedQuota)
			var channel model.Channel
			require.NoError(t, model.DB.First(&channel, tt.info.ChannelId).Error)
			require.Equal(t, int64(tt.want), channel.UsedQuota)

			require.NoError(t, model.DB.Exec("UPDATE users SET used_quota = 0, request_count = 0 WHERE id = ?", tt.info.UserId).Error)
			require.NoError(t, model.DB.Exec("UPDATE channels SET used_quota = 0 WHERE id = ?", tt.info.ChannelId).Error)
			require.NoError(t, model.LOG_DB.Exec("DELETE FROM logs").Error)
		})
	}
}

func TestAmbiguousTextBillingDoesNotRecordConsumptionWhenSettlementFails(t *testing.T) {
	truncate(t)
	seedUser(t, 101, 0)
	seedChannel(t, 201)
	billing := &recordingTaskBillingSettler{settleErr: errors.New("settlement failed")}
	relayInfo := &relaycommon.RelayInfo{
		UserId: 101, TokenId: 301, UsingGroup: "default", OriginModelName: "gpt-test",
		StartTime: time.Now(), Billing: billing,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 201},
		PriceData:   types.PriceData{QuotaToPreConsume: 1250, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}},
	}
	attempt := relayInfo.BeginUpstreamRequestAttempt()
	attempt.MarkRequestWritten()
	attempt.MarkAmbiguousIfPotentiallySent()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	err := SettleAmbiguousTextBilling(ctx, relayInfo)

	require.Error(t, err)
	require.Equal(t, []int{1250}, billing.settled)
	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ?", relayInfo.UserId).Find(&logs).Error)
	require.Empty(t, logs)
	var user model.User
	require.NoError(t, model.DB.First(&user, relayInfo.UserId).Error)
	require.Zero(t, user.UsedQuota)
	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, relayInfo.ChannelId).Error)
	require.Zero(t, channel.UsedQuota)
}

func TestAcceptedStreamBillingDoesNotRecordConsumptionWhenSettlementFails(t *testing.T) {
	originalEnabled := constant.StreamUsageDrainEnabled
	constant.StreamUsageDrainEnabled = true
	t.Cleanup(func() { constant.StreamUsageDrainEnabled = originalEnabled })
	truncate(t)
	seedUser(t, 101, 0)
	seedChannel(t, 201)
	billing := &recordingTaskBillingSettler{preConsumed: 1250, settleErr: errors.New("settlement failed")}
	relayInfo := &relaycommon.RelayInfo{
		UserId: 101, TokenId: 301, UsingGroup: "default", OriginModelName: "gpt-test",
		StartTime: time.Now(), Billing: billing, FinalPreConsumedQuota: 1250,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 201},
		PriceData: types.PriceData{
			ModelRatio: 1, CompletionRatio: 1, QuotaToPreConsume: 1250,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
	}
	relayInfo.SetEstimatePromptTokens(400)
	relayInfo.EnableStreamRecovery()
	relayInfo.MarkStreamAccepted()
	t.Cleanup(relayInfo.FinishStreamRecovery)
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	err := SettleAcceptedTextBilling(ctx, relayInfo, &dto.Usage{})

	require.Error(t, err)
	require.Equal(t, []int{1250}, billing.settled)
	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ?", relayInfo.UserId).Find(&logs).Error)
	require.Empty(t, logs)
	var user model.User
	require.NoError(t, model.DB.First(&user, relayInfo.UserId).Error)
	require.Zero(t, user.UsedQuota)
	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, relayInfo.ChannelId).Error)
	require.Zero(t, channel.UsedQuota)
}

func TestUnconfirmedTerminalUsageDoesNotRunTieredSettlement(t *testing.T) {
	billing := &recordingTaskBillingSettler{preConsumed: 500}
	relayInfo := &relaycommon.RelayInfo{
		UserId:                        1,
		TokenId:                       1,
		OriginModelName:               "gpt-test",
		Billing:                       billing,
		FinalPreConsumedQuota:         500,
		PreservePreConsumedQuota:      true,
		StreamTerminalMarkersRequired: true,
		PriceData: types.PriceData{
			ModelRatio:        1,
			CompletionRatio:   1,
			QuotaToPreConsume: 500,
			GroupRatioInfo:    types.GroupRatioInfo{GroupRatio: 1},
		},
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:              "tiered_expr",
			ExprString:               `tier("local_estimate", p * 1000 + c * 1000)`,
			GroupRatio:               1,
			EstimatedQuotaAfterGroup: 500,
			QuotaPerUnit:             500_000,
		},
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 1},
	}
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	PostTextConsumeQuota(ctx, relayInfo, &dto.Usage{PromptTokens: 10_000, CompletionTokens: 10_000}, nil)

	require.Equal(t, []int{20_000}, billing.settled)
}

func TestIncompleteResponsesUsageAlwaysUsesPreConsumedQuotaFloor(t *testing.T) {
	for _, reason := range []relaycommon.StreamEndReason{
		relaycommon.StreamEndReasonClientGone,
		relaycommon.StreamEndReasonHandlerStop,
		relaycommon.StreamEndReasonEOF,
		relaycommon.StreamEndReasonTimeout,
		relaycommon.StreamEndReasonScannerErr,
		relaycommon.StreamEndReasonPingFail,
	} {
		t.Run(string(reason), func(t *testing.T) {
			relayInfo := &relaycommon.RelayInfo{
				IsStream:                      true,
				StreamStatus:                  streamStatusForAffinityUsageTest(reason, true),
				StreamTerminalMarkersRequired: true,
				PreservePreConsumedQuota:      true,
				FinalPreConsumedQuota:         1250,
			}

			quota, preserved := applyPreConsumedQuotaFloor(relayInfo, 400)

			require.Equal(t, 1250, quota)
			require.True(t, preserved)
			require.False(t, shouldObserveConfirmedChannelAffinityUsage(nil, relayInfo, &dto.Usage{
				PromptTokens: 400,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: 0,
				},
			}))
		})
	}
}
