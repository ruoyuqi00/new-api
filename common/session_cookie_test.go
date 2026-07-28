package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitSessionCookieSettings(t *testing.T) {
	tests := []struct {
		name       string
		secure     string
		trusted    string
		wantSecure bool
		wantURLs   []string
		wantError  bool
	}{
		{name: "disabled by default"},
		{name: "secure https entries", secure: "true", trusted: "https://yuaiapi.com, https://global.yuaiapi.com", wantSecure: true, wantURLs: []string{"https://yuaiapi.com", "https://global.yuaiapi.com"}},
		{name: "secure requires trusted URL", secure: "true", wantError: true},
		{name: "trusted URL requires secure", trusted: "https://yuaiapi.com", wantError: true},
		{name: "http is rejected", secure: "true", trusted: "http://yuaiapi.com", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("SESSION_COOKIE_SECURE", test.secure)
			t.Setenv("SESSION_COOKIE_TRUSTED_URL", test.trusted)
			err := InitSessionCookieSettings()
			if test.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.wantSecure, SessionCookieSecure)
			assert.Equal(t, test.wantURLs, SessionCookieTrustedURLs)
		})
	}
}
