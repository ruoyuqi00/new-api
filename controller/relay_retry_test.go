package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
