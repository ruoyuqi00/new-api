package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInputModerationEnabledRequiresExplicitFlag(t *testing.T) {
	t.Setenv("INPUT_MODERATION_ENABLED", "")
	require.False(t, InputModerationEnabled())

	t.Setenv("INPUT_MODERATION_ENABLED", "true")
	require.True(t, InputModerationEnabled())
}

func TestInputModerationCheckerReturnsAllowedResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-moderation-key", r.Header.Get("Authorization"))

		var request struct {
			Model string `json:"model"`
			Input string `json:"input"`
		}
		require.NoError(t, common.DecodeJson(r.Body, &request))
		assert.Equal(t, "omni-moderation-latest", request.Model)
		assert.Equal(t, "ordinary user input", request.Input)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"id":"modr-allowed","model":"omni-moderation-latest","results":[{"flagged":false,"categories":{"violence":false}}]}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	checker := inputModerationChecker{
		endpoint:   server.URL,
		apiKey:     "test-moderation-key",
		model:      "omni-moderation-latest",
		timeout:    time.Second,
		httpClient: server.Client(),
	}
	result, err := checker.Check(context.Background(), "ordinary user input")

	require.NoError(t, err)
	assert.False(t, result.Flagged)
	assert.Equal(t, "omni-moderation-latest", result.Model)
	assert.Empty(t, result.Categories)
}

func TestInputModerationCheckerSkipsEmptyInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("empty input must not call the moderation provider")
	}))
	t.Cleanup(server.Close)

	checker := inputModerationChecker{
		endpoint:   server.URL,
		apiKey:     "test-moderation-key",
		model:      "omni-moderation-latest",
		timeout:    time.Second,
		httpClient: server.Client(),
	}
	result, err := checker.Check(context.Background(), "   ")

	require.NoError(t, err)
	assert.Equal(t, InputModerationResult{}, result)
}

func TestInputModerationCheckerReturnsSortedFlaggedCategories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"id":"modr-flagged","model":"omni-moderation-2024-09-26","results":[{"flagged":true,"categories":{"violence":true,"harassment":false,"illicit":true}}]}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	checker := inputModerationChecker{
		endpoint:   server.URL,
		apiKey:     "test-moderation-key",
		model:      "omni-moderation-latest",
		timeout:    time.Second,
		httpClient: server.Client(),
	}
	result, err := checker.Check(context.Background(), "policy test input")

	require.NoError(t, err)
	assert.True(t, result.Flagged)
	assert.Equal(t, "omni-moderation-2024-09-26", result.Model)
	assert.Equal(t, []string{"illicit", "violence"}, result.Categories)
}

func TestInputModerationCheckerRejectsInvalidProviderResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		errText    string
	}{
		{name: "non success status", statusCode: http.StatusTooManyRequests, body: `provider-secret-body`, errText: "status 429"},
		{name: "malformed json", statusCode: http.StatusOK, body: `{`, errText: "decode"},
		{name: "empty results", statusCode: http.StatusOK, body: `{"model":"omni-moderation-latest","results":[]}`, errText: "no results"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.statusCode)
				_, err := w.Write([]byte(test.body))
				require.NoError(t, err)
			}))
			t.Cleanup(server.Close)

			checker := inputModerationChecker{
				endpoint:   server.URL,
				apiKey:     "test-moderation-key",
				model:      "omni-moderation-latest",
				timeout:    time.Second,
				httpClient: server.Client(),
			}
			_, err := checker.Check(context.Background(), "input-must-not-appear-in-error")

			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), test.errText)
			assert.NotContains(t, err.Error(), test.body)
			assert.NotContains(t, err.Error(), "input-must-not-appear-in-error")
			assert.NotContains(t, err.Error(), "test-moderation-key")
		})
	}
}

func TestInputModerationCheckerHonorsTimeout(t *testing.T) {
	releaseHandler := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-releaseHandler
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(releaseHandler) })

	checker := inputModerationChecker{
		endpoint:   server.URL,
		apiKey:     "test-moderation-key",
		model:      "omni-moderation-latest",
		timeout:    10 * time.Millisecond,
		httpClient: server.Client(),
	}
	_, err := checker.Check(context.Background(), "timeout input")

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
