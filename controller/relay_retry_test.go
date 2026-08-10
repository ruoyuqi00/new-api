package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newRelayRetryTestContext(t *testing.T, canceled bool) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestContext, cancel := context.WithCancel(context.Background())
	if canceled {
		cancel()
	} else {
		t.Cleanup(cancel)
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestContext)
	return c
}

func retryableRelayTestError(statusCode int) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		errors.New("upstream unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		statusCode,
	)
}

func TestShouldRetryAllowsRetryableStatusWithActiveClient(t *testing.T) {
	c := newRelayRetryTestContext(t, false)

	require.True(t, shouldRetry(c, retryableRelayTestError(http.StatusServiceUnavailable), 1))
}

func TestShouldRetryStopsRetryableStatusAfterClientCancellation(t *testing.T) {
	c := newRelayRetryTestContext(t, true)

	require.False(t, shouldRetry(c, retryableRelayTestError(http.StatusServiceUnavailable), 1))
}

func TestShouldRetryStopsChannelErrorAfterClientCancellation(t *testing.T) {
	c := newRelayRetryTestContext(t, true)
	channelErr := types.NewError(
		errors.New("provider account unavailable"),
		types.ErrorCodeChannelNoAvailableKey,
	)

	require.False(t, shouldRetry(c, channelErr, 1))
}

func TestShouldRetryStopsContextCanceledError(t *testing.T) {
	c := newRelayRetryTestContext(t, false)
	canceledErr := types.NewError(context.Canceled, types.ErrorCodeDoRequestFailed)

	require.False(t, shouldRetry(c, canceledErr, 1))
}

func TestShouldCommitChannelAffinityRequiresNormalRelayCompletion(t *testing.T) {
	tests := []struct {
		name       string
		canceled   bool
		relayInfo  *relaycommon.RelayInfo
		wantCommit bool
	}{
		{name: "non-stream success", relayInfo: &relaycommon.RelayInfo{}, wantCommit: true},
		{name: "client canceled", canceled: true, relayInfo: &relaycommon.RelayInfo{}},
		{
			name: "stream completed",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:     true,
				StreamStatus: relayStreamStatusForAffinityTest(relaycommon.StreamEndReasonDone, false),
			},
			wantCommit: true,
		},
		{
			name: "stream client gone",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:     true,
				StreamStatus: relayStreamStatusForAffinityTest(relaycommon.StreamEndReasonClientGone, false),
			},
		},
		{
			name: "stream ended with errors",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:     true,
				StreamStatus: relayStreamStatusForAffinityTest(relaycommon.StreamEndReasonEOF, true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newRelayRetryTestContext(t, tt.canceled)
			require.Equal(t, tt.wantCommit, shouldCommitChannelAffinity(c, tt.relayInfo))
		})
	}
}

func TestPrepareRelayRetryPreservesCurrentAffinity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestBody := `{"model":"gpt-5","prompt_cache_key":"retry-preserves-affinity"}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(requestBody))
	c.Request.Header.Set("Content-Type", "application/json")

	_, found := service.GetPreferredChannelByAffinity(c, "gpt-5", "gpt-pro")
	require.False(t, found)
	service.MarkChannelAffinityRequestSucceeded(c)
	service.RecordChannelAffinity(c, 9527)
	t.Cleanup(func() { service.ClearCurrentChannelAffinityCache(c) })

	retry := 0
	retryParam := &service.RetryParam{Ctx: c, Retry: &retry}
	common.SetContextKey(c, constant.ContextKeyProviderAccountId, 0)
	prepareRelayRetry(c, retryParam, 9527, false)

	nextRecorder := httptest.NewRecorder()
	nextCtx, _ := gin.CreateTestContext(nextRecorder)
	nextCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(requestBody))
	nextCtx.Request.Header.Set("Content-Type", "application/json")
	channelID, found := service.GetPreferredChannelByAffinity(nextCtx, "gpt-5", "gpt-pro")
	require.True(t, found)
	require.Equal(t, 9527, channelID)
	require.Contains(t, retryParam.ChannelSelectionOptions().SkipChannelIDs, 9527)
}

func relayStreamStatusForAffinityTest(reason relaycommon.StreamEndReason, withError bool) *relaycommon.StreamStatus {
	status := relaycommon.NewStreamStatus()
	status.SetEndReason(reason, nil)
	if withError {
		status.RecordError("upstream stream ended before completion")
	}
	return status
}
