package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderAccountUpstreamErrorPreservesUsefulDiagnostics(t *testing.T) {
	tests := []struct {
		name        string
		payload     any
		status      int
		errorCode   string
		messagePart string
	}{
		{
			name: "nested upstream code",
			payload: map[string]interface{}{
				"error": map[string]interface{}{"code": "subscription_inactive", "message": "subscription is inactive"},
			},
			status:      http.StatusForbidden,
			errorCode:   "subscription_inactive",
			messagePart: "HTTP 403: subscription is inactive",
		},
		{
			name:        "rate limit fallback",
			payload:     map[string]interface{}{},
			status:      http.StatusTooManyRequests,
			errorCode:   "http_429",
			messagePart: "rate limited",
		},
		{
			name:        "top level detail",
			payload:     map[string]interface{}{"detail": "account deactivated"},
			status:      http.StatusUnauthorized,
			errorCode:   "http_401",
			messagePart: "account deactivated",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errorCode, message := providerAccountUpstreamError(test.payload, test.status)
			assert.Equal(t, test.errorCode, errorCode)
			assert.Contains(t, message, test.messagePart)
		})
	}
}
