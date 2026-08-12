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

func TestShouldRetryStopsAfterIncompleteResponsesStream(t *testing.T) {
	for _, reason := range []relaycommon.StreamEndReason{
		relaycommon.StreamEndReasonClientGone,
		relaycommon.StreamEndReasonHandlerStop,
		relaycommon.StreamEndReasonEOF,
		relaycommon.StreamEndReasonTimeout,
		relaycommon.StreamEndReasonScannerErr,
	} {
		t.Run(string(reason), func(t *testing.T) {
			c := newRelayRetryTestContext(t, false)
			relayInfo := &relaycommon.RelayInfo{
				IsStream:                      true,
				StreamStatus:                  relayStreamStatusForAffinityTest(reason, true),
				StreamTerminalMarkersRequired: true,
				ReceivedResponseCount:         1,
			}

			require.False(t, shouldRetryRelayOutcome(c, relayInfo, retryableRelayTestError(http.StatusServiceUnavailable), 1))
		})
	}
}

func TestShouldRetryRelayOutcomePreservesSafePreResponseRetry(t *testing.T) {
	c := newRelayRetryTestContext(t, false)
	relayInfo := &relaycommon.RelayInfo{
		IsStream:                      true,
		StreamTerminalMarkersRequired: true,
	}

	require.True(t, shouldRetryRelayOutcome(c, relayInfo, retryableRelayTestError(http.StatusServiceUnavailable), 1))
}

func TestShouldRetryRelayOutcomeStopsAfterAmbiguousWrittenSubmission(t *testing.T) {
	c := newRelayRetryTestContext(t, false)
	relayInfo := &relaycommon.RelayInfo{}
	attempt := relayInfo.BeginUpstreamRequestAttempt()
	attempt.MarkRequestWritten()
	attempt.MarkAmbiguousIfPotentiallySent()

	require.False(t, shouldRetryRelayOutcome(c, relayInfo, retryableRelayTestError(http.StatusServiceUnavailable), 1))
	require.False(t, shouldRefundRelayFailure(relayInfo))
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
			name: "stream without terminal status",
			relayInfo: &relaycommon.RelayInfo{
				IsStream: true,
			},
		},
		{
			name: "responses stream completed",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:                      true,
				RelayFormat:                   types.RelayFormatOpenAIResponses,
				StreamStatus:                  relayStreamStatusForAffinityTest(relaycommon.StreamEndReasonDone, false),
				StreamTerminalMarkersRequired: true,
				StreamTerminalSuccess:         true,
			},
			wantCommit: true,
		},
		{
			name: "responses stream incomplete",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:                      true,
				RelayFormat:                   types.RelayFormatOpenAIResponses,
				StreamStatus:                  relayStreamStatusForAffinityTest(relaycommon.StreamEndReasonEOF, false),
				StreamTerminalMarkersRequired: true,
			},
		},
		{
			name: "responses adapter with independent terminal handling",
			relayInfo: &relaycommon.RelayInfo{
				IsStream:     true,
				RelayFormat:  types.RelayFormatOpenAIResponses,
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

func newResponseChainCommitTestContext(t *testing.T, body string, tokenID int, canceled bool) *gin.Context {
	t.Helper()
	c := newRelayRetryTestContext(t, canceled)
	requestContext := c.Request.Context()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body)).WithContext(requestContext)
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyTokenId, tokenID)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "gptpro")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-5")
	return c
}

func TestCommitResponseChainAffinityOutcome(t *testing.T) {
	tests := []struct {
		name       string
		canceled   bool
		streamEnd  relaycommon.StreamEndReason
		terminal   bool
		observed   bool
		responseID bool
		wantFound  bool
	}{
		{name: "completed", streamEnd: relaycommon.StreamEndReasonDone, terminal: true, observed: true, responseID: true, wantFound: true},
		{name: "client gone after response created", streamEnd: relaycommon.StreamEndReasonClientGone, observed: true, responseID: true, wantFound: true},
		{name: "canceled context after response created", canceled: true, streamEnd: relaycommon.StreamEndReasonClientGone, observed: true, responseID: true, wantFound: true},
		{name: "eof after response created", streamEnd: relaycommon.StreamEndReasonEOF, observed: true, responseID: true, wantFound: true},
		{name: "context canceled before response created", canceled: true, streamEnd: relaycommon.StreamEndReasonClientGone},
		{name: "missing real response id", streamEnd: relaycommon.StreamEndReasonEOF},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseID := "resp-controller-" + strings.ReplaceAll(tt.name, " ", "-")
			tokenID := 8300 + index
			first := newResponseChainCommitTestContext(t, `{"model":"gpt-5","input":"first"}`, tokenID, tt.canceled)
			_, found := service.GetPreferredChannelByAffinity(first, "gpt-5", "gptpro")
			require.False(t, found)
			actualResponseID := ""
			if tt.responseID {
				actualResponseID = responseID
			}
			relayInfo := &relaycommon.RelayInfo{
				IsStream:                          true,
				RelayFormat:                       types.RelayFormatOpenAIResponses,
				StreamStatus:                      relayStreamStatusForAffinityTest(tt.streamEnd, false),
				StreamTerminalMarkersRequired:     true,
				StreamTerminalSuccess:             tt.terminal,
				ChannelAffinityResponseID:         actualResponseID,
				ChannelAffinityResponseIDObserved: tt.observed,
			}

			commitChannelAffinityOutcome(first, relayInfo, 9300+index)
			service.RecordChannelAffinity(first, 9300+index)

			nextBody := `{"model":"gpt-5","previous_response_id":"` + responseID + `","input":"next"}`
			next := newResponseChainCommitTestContext(t, nextBody, tokenID, false)
			channelID, found := service.GetPreferredChannelByAffinity(next, "gpt-5", "gptpro")
			require.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				require.Equal(t, 9300+index, channelID)
				t.Cleanup(func() { service.ClearCurrentChannelAffinityCache(next) })
			} else {
				require.Zero(t, channelID)
			}
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
