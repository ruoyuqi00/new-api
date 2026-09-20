package sora

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMoonTestContext(t *testing.T, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx
}

func TestMoonRequestPreservesContentAndAcceptsNumericSeconds(t *testing.T) {
	ctx := newMoonTestContext(t, `{
		"model":"seedance-2-5-official",
		"content":[
			{"type":"text","text":"camera pushes forward"},
			{"type":"video_url","video_url":{"url":"https://example.com/ref.mp4"},"role":"reference_video","durationSeconds":3}
		],
		"seconds":5,
		"resolution":"1080p",
		"ratio":"16:9"
	}`)
	ctx.Request.Header.Set("Idempotency-Key", "moon-order-123")
	info := &relaycommon.RelayInfo{
		OriginModelName: "seedance-2-5-official",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public_123"},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://moon.sixai.cc",
			UpstreamModelName: "seedance-2-5-official",
		},
	}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
	body, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	upstreamBody, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"model":"seedance-2-5-official",
		"content":[
			{"type":"text","text":"camera pushes forward"},
			{"type":"video_url","video_url":{"url":"https://example.com/ref.mp4"},"role":"reference_video","durationSeconds":3}
		],
		"seconds":5,
		"resolution":"1080p",
		"ratio":"16:9"
	}`, string(upstreamBody))

	req := httptest.NewRequest(http.MethodPost, "https://moon.sixai.cc/v1/videos", nil)
	require.NoError(t, adaptor.BuildRequestHeader(ctx, req, info))
	assert.Equal(t, "moon-order-123", req.Header.Get("Idempotency-Key"))
}

func TestMoonSeedanceEstimateUsesResolutionVideoReferenceAndTokenReservation(t *testing.T) {
	ctx := newMoonTestContext(t, `{
		"model":"seedance-2-0-official",
		"prompt":"camera pushes forward",
		"duration":5,
		"resolution":"1080p",
		"videos":[{"url":"https://example.com/ref.mp4","durationSeconds":3}]
	}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "seedance-2-0-official",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelBaseUrl: "https://moon.sixai.cc"},
	}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
	ratios := adaptor.EstimateBilling(ctx, info)

	assert.InDelta(t, 24.8/36.8, ratios[moonRateRatioKey], 1e-9)
	assert.InDelta(t, 1.5552, ratios[moonTokenReservationRatioKey], 1e-9)
}

func TestMoonH3EstimateUsesFixedResolutionPriceWithoutTimeDiscount(t *testing.T) {
	ctx := newMoonTestContext(t, `{
		"model":"minimax-h3",
		"prompt":"camera pushes forward",
		"workflow_id":"cf-multi-reference",
		"seconds":4,
		"size":"4K",
		"aspect_ratio":"16:9"
	}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "minimax-h3",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelBaseUrl: "https://moon.sixai.cc"},
	}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
	ratios := adaptor.EstimateBilling(ctx, info)

	assert.Equal(t, 4.0, ratios["seconds"])
	assert.InDelta(t, 3.6, ratios[moonResolutionRatioKey], 1e-9)
	assert.NotContains(t, ratios, "hour")
	assert.NotContains(t, ratios, "discount")
}

func TestMoonCompletedSeedanceTaskUsesAuthoritativeTokenUsage(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://moon.sixai.cc"}
	result, err := adaptor.ParseTaskResult([]byte(`{
		"id":"tvid_123",
		"status":"completed",
		"data":[{"url":"https://cdn.example.com/video.mp4"}],
		"usage":{"total_tokens":100000}
	}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	assert.Equal(t, "https://cdn.example.com/video.mp4", result.Url)
	assert.Equal(t, 100000, result.TotalTokens)

	task := &model.Task{
		Properties: model.Properties{OriginModelName: "seedance-2-0-official"},
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
			OriginModelName: "seedance-2-0-official",
			ModelRatio:      18.4,
			GroupRatio:      0.3,
			OtherRatios: map[string]float64{
				moonRateRatioKey:             24.8 / 36.8,
				moonTokenReservationRatioKey: 1.5552,
			},
		}},
	}
	assert.Equal(t, 372000, adaptor.AdjustBillingOnComplete(task, result))
}

func TestMoonPendingSettlementStatusesRemainInProgress(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://moon.sixai.cc"}
	for _, status := range []string{"usage_pending", "funding_pending", "commit_pending", "submitting_unknown", "reservation_unknown"} {
		t.Run(status, func(t *testing.T) {
			result, err := adaptor.ParseTaskResult([]byte(`{"status":"` + status + `"}`))
			require.NoError(t, err)
			assert.Equal(t, model.TaskStatusInProgress, result.Status)
		})
	}
}

func TestMoonCompletedSeedanceWithoutAuthoritativeUsageKeepsPolling(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://moon.sixai.cc"}
	result, err := adaptor.ParseTaskResult([]byte(`{
		"model":"seedance-2-0-official",
		"status":"completed",
		"data":[{"url":"https://cdn.example.com/video.mp4"}]
	}`))

	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusInProgress, result.Status)
	assert.Empty(t, result.Url)
	assert.Zero(t, result.TotalTokens)
}

func TestMoonSeedanceMinusOneDurationReservesModelMaximum(t *testing.T) {
	ctx := newMoonTestContext(t, `{
		"model":"seedance-2-5-official",
		"prompt":"camera pushes forward",
		"duration":-1,
		"resolution":"720p"
	}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "seedance-2-5-official",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelBaseUrl: "https://moon.sixai.cc"},
	}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
	ratios := adaptor.EstimateBilling(ctx, info)

	// 30 seconds * 1280 * 720 * 24 fps / 1024 tokens, converted to the
	// task pre-consume multiplier (tokens * 2 / QuotaPerUnit).
	assert.InDelta(t, 2.592, ratios[moonTokenReservationRatioKey], 1e-9)
}

func TestMoonSeedanceTokenRateTableMatchesApprovedScreenshot(t *testing.T) {
	tests := []struct {
		model      string
		resolution string
		hasVideo   bool
		want       float64
	}{
		{"seedance-2-0-mini-official", "480p", false, 11.5},
		{"seedance-2-0-mini-official", "720p", true, 7},
		{"seedance-2-0-fast-official", "480p", false, 24.05},
		{"seedance-2-0-fast-official", "720p", true, 14.3},
		{"seedance-2-0-official", "480p", false, 36.8},
		{"seedance-2-0-official", "720p", true, 22.4},
		{"seedance-2-0-official", "1080p", false, 40.8},
		{"seedance-2-0-official", "1080p", true, 24.8},
		{"seedance-2-0-official", "4k", false, 20.8},
		{"seedance-2-0-official", "4k", true, 12.8},
		{"seedance-2-5-official", "720p", false, 59.5},
		{"seedance-2-5-official", "720p", true, 35.7},
		{"seedance-2-5-official", "1080p", false, 65.45},
		{"seedance-2-5-official", "1080p", true, 39.1},
	}

	for _, test := range tests {
		t.Run(test.model+"/"+test.resolution, func(t *testing.T) {
			got, _, ok := moonSeedanceTokenRate(test.model, test.resolution, test.hasVideo)
			require.True(t, ok)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestMoonH3RateTableMatchesApprovedScreenshot(t *testing.T) {
	tests := map[string]float64{
		"864x480":   0.10,
		"1376x768":  0.16,
		"1920x1088": 0.18,
		"2K":        0.26,
		"4K":        0.36,
	}
	for size, want := range tests {
		t.Run(size, func(t *testing.T) {
			assert.Equal(t, want, moonH3ResolutionRate(size, ""))
		})
	}
}

func TestMoonVideoFetchDoesNotExposeUpstreamBilling(t *testing.T) {
	// Stored-task fetch creates a fresh adaptor without Init. The model itself
	// must still select the Moon-safe response shape.
	adaptor := &TaskAdaptor{}
	task := &model.Task{
		TaskID:     "task_public_123",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		CreatedAt:  100,
		UpdatedAt:  200,
		Properties: model.Properties{OriginModelName: "seedance-2-5-official"},
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://cdn.example.com/video.mp4",
		},
		Data: []byte(`{"id":"upstream_secret","status":"completed","billing":{"charged_amount":9.9}}`),
	}

	body, err := adaptor.ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"id":"task_public_123",
		"object":"video",
		"model":"seedance-2-5-official",
		"status":"completed",
		"progress":100,
		"created_at":100,
		"completed_at":200,
		"metadata":{"url":"https://cdn.example.com/video.mp4"}
	}`, string(body))
	assert.NotContains(t, string(body), "billing")
	assert.NotContains(t, string(body), "upstream_secret")
}

func TestMoonRejectsInvalidH3Duration(t *testing.T) {
	for _, seconds := range []string{"0", "-1", "3", "16"} {
		t.Run(seconds, func(t *testing.T) {
			ctx := newMoonTestContext(t, `{"model":"minimax-h3","prompt":"camera pushes forward","seconds":`+seconds+`,"size":"864x480"}`)
			info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://moon.sixai.cc"}}
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)

			taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
			require.NotNil(t, taskErr)
			assert.Equal(t, "invalid_seconds", taskErr.Code)
		})
	}
}

func TestMoonRejectsConflictingDurationAliases(t *testing.T) {
	ctx := newMoonTestContext(t, `{"model":"minimax-h3","prompt":"camera pushes forward","duration":4,"seconds":5,"size":"864x480"}`)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://moon.sixai.cc"}}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_seconds", taskErr.Code)
}

func TestMoonRequiresExplicitH3Duration(t *testing.T) {
	ctx := newMoonTestContext(t, `{"model":"minimax-h3","prompt":"camera pushes forward","size":"864x480"}`)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://moon.sixai.cc"}}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_seconds", taskErr.Code)
}

func TestMoonValidatesSeedanceDurationByModel(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		duration  int
		wantError bool
	}{
		{name: "2.0 accepts lower bound", modelName: "seedance-2-0-official", duration: 4},
		{name: "2.0 accepts upper bound", modelName: "seedance-2-0-official", duration: 15},
		{name: "2.0 accepts automatic duration", modelName: "seedance-2-0-official", duration: -1},
		{name: "2.0 rejects above upper bound", modelName: "seedance-2-0-official", duration: 16, wantError: true},
		{name: "2.5 accepts upper bound", modelName: "seedance-2-5-official", duration: 30},
		{name: "2.5 rejects above upper bound", modelName: "seedance-2-5-official", duration: 31, wantError: true},
		{name: "2.5 rejects below lower bound", modelName: "seedance-2-5-official", duration: 3, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"model":%q,"prompt":"camera pushes forward","duration":%d,"resolution":"720p"}`, test.modelName, test.duration)
			ctx := newMoonTestContext(t, body)
			info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://moon.sixai.cc"}}
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)

			taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
			if test.wantError {
				require.NotNil(t, taskErr)
				assert.Equal(t, "invalid_seconds", taskErr.Code)
				return
			}
			require.Nil(t, taskErr)
		})
	}
}

func TestMoonPerSecondModelsUseApprovedRates(t *testing.T) {
	tests := []struct {
		name                string
		body                string
		wantSeconds         float64
		wantResolutionRatio float64
	}{
		{
			name:                "Wan standard includes reference video duration",
			body:                `{"model":"wan3.0-video","prompt":"camera pushes forward","duration":5,"resolution":"1080p","reference_videos":[{"url":"https://example.com/ref.mp4","durationSeconds":3}]}`,
			wantSeconds:         8,
			wantResolutionRatio: 0.72 / 0.27,
		},
		{
			name:                "Wan prime uses screenshot rate",
			body:                `{"model":"wan3.0-video-prime","prompt":"camera pushes forward","duration":5,"resolution":"720p"}`,
			wantSeconds:         5,
			wantResolutionRatio: 0.54 / 0.40,
		},
		{
			name:                "Seedance 2.0 PT uses output duration only",
			body:                `{"model":"seedance2.0-9-3-3-PT","prompt":"camera pushes forward","duration":6,"resolution":"720p","reference_videos":[{"url":"https://example.com/ref.mp4","durationSeconds":3}]}`,
			wantSeconds:         6,
			wantResolutionRatio: 0.42 / 0.34,
		},
		{
			name:                "Seedance 2.5 PT accepts arbitrary integer duration",
			body:                `{"model":"seedance2.5-30-10-10-PT","prompt":"camera pushes forward","duration":7,"resolution":"480p"}`,
			wantSeconds:         7,
			wantResolutionRatio: 1,
		},
		{
			name:                "Seedance Fast PT uses live upstream rate",
			body:                `{"model":"seedance2.0-fast-PT","prompt":"camera pushes forward","duration":9,"resolution":"720p"}`,
			wantSeconds:         9,
			wantResolutionRatio: 0.36 / 0.30,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := newMoonTestContext(t, test.body)
			info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://moon.sixai.cc"}}
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)

			require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
			ratios := adaptor.EstimateBilling(ctx, info)
			assert.Equal(t, test.wantSeconds, ratios["seconds"])
			assert.InDelta(t, test.wantResolutionRatio, ratios[moonResolutionRatioKey], 1e-9)
		})
	}
}

func TestMoonGrokVideoAcceptsPerRequestBilling(t *testing.T) {
	ctx := newMoonTestContext(t, `{"model":"grok-v1.5-video","prompt":"camera pushes forward","seconds":"6","size":"1080p"}`)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://moon.sixai.cc"}}
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
	assert.Empty(t, adaptor.EstimateBilling(ctx, info))
}

func TestMoonValidatesNewModelDurationAndResolution(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{name: "Wan duration above maximum", body: `{"model":"wan3.0-video","prompt":"x","duration":31,"resolution":"720p"}`, code: "invalid_seconds"},
		{name: "Wan unsupported resolution", body: `{"model":"wan3.0-video","prompt":"x","duration":5,"resolution":"4k"}`, code: "unsupported_pricing_tier"},
		{name: "Seedance 2.0 PT duration above live maximum", body: `{"model":"seedance2.0-9-3-3-PT","prompt":"x","duration":16,"resolution":"720p"}`, code: "invalid_seconds"},
		{name: "Seedance 2.5 PT duration above maximum", body: `{"model":"seedance2.5-30-10-10-PT","prompt":"x","duration":31,"resolution":"720p"}`, code: "invalid_seconds"},
		{name: "Grok duration below minimum", body: `{"model":"grok-v1.5-video","prompt":"x","duration":3,"resolution":"720p"}`, code: "invalid_seconds"},
		{name: "Grok unsupported resolution", body: `{"model":"grok-v1.5-video","prompt":"x","duration":6,"resolution":"480p"}`, code: "unsupported_pricing_tier"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := newMoonTestContext(t, test.body)
			info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://moon.sixai.cc"}}
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)

			taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
			require.NotNil(t, taskErr)
			assert.Equal(t, test.code, taskErr.Code)
		})
	}
}

func TestMoonRejectsUnsupportedPricedResolution(t *testing.T) {
	tests := []string{
		`{"model":"seedance-2-5-official","prompt":"camera pushes forward","duration":5,"resolution":"480p"}`,
		`{"model":"minimax-h3","prompt":"camera pushes forward","seconds":5,"size":"not-a-tier"}`,
	}
	for _, body := range tests {
		ctx := newMoonTestContext(t, body)
		info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://moon.sixai.cc"}}
		adaptor := &TaskAdaptor{}
		adaptor.Init(info)

		taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
		require.NotNil(t, taskErr)
		assert.Equal(t, "unsupported_pricing_tier", taskErr.Code)
	}
}
