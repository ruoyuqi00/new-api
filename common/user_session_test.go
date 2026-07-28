package common

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitUserSessionSettingsUsesPositiveFallbacksAndClampsWindow(t *testing.T) {
	previousActiveLimit := UserSessionActiveLimit
	previousIssuanceLimit := UserSessionIssuanceLimit
	previousIssuanceWindow := UserSessionIssuanceWindowSeconds
	previousRevokedRetention := UserSessionRevokedRetentionDays
	previousAlertThreshold := UserSessionHourlyAlertThreshold
	t.Cleanup(func() {
		UserSessionActiveLimit = previousActiveLimit
		UserSessionIssuanceLimit = previousIssuanceLimit
		UserSessionIssuanceWindowSeconds = previousIssuanceWindow
		UserSessionRevokedRetentionDays = previousRevokedRetention
		UserSessionHourlyAlertThreshold = previousAlertThreshold
	})

	t.Setenv("USER_SESSION_ACTIVE_LIMIT", "0")
	t.Setenv("USER_SESSION_ISSUANCE_LIMIT", "-2")
	t.Setenv("USER_SESSION_ISSUANCE_WINDOW_SECONDS", "invalid")
	t.Setenv("USER_SESSION_REVOKED_RETENTION_DAYS", "0")
	t.Setenv("USER_SESSION_HOURLY_ALERT_THRESHOLD", "-1")
	initUserSessionSettings()

	assert.Equal(t, DefaultUserSessionActiveLimit, UserSessionActiveLimit)
	assert.Equal(t, DefaultUserSessionIssuanceLimit, UserSessionIssuanceLimit)
	assert.Equal(t, int64(DefaultUserSessionIssuanceWindowSeconds), UserSessionIssuanceWindowSeconds)
	assert.Equal(t, DefaultUserSessionRevokedRetentionDays, UserSessionRevokedRetentionDays)
	assert.Equal(t, DefaultUserSessionHourlyAlertThreshold, UserSessionHourlyAlertThreshold)

	t.Setenv("USER_SESSION_ACTIVE_LIMIT", "12")
	t.Setenv("USER_SESSION_ISSUANCE_LIMIT", "34")
	t.Setenv("USER_SESSION_ISSUANCE_WINDOW_SECONDS", "172800")
	t.Setenv("USER_SESSION_REVOKED_RETENTION_DAYS", "1")
	t.Setenv("USER_SESSION_HOURLY_ALERT_THRESHOLD", "56")
	initUserSessionSettings()

	assert.Equal(t, 12, UserSessionActiveLimit)
	assert.Equal(t, 34, UserSessionIssuanceLimit)
	assert.Equal(t, int64(24*60*60), UserSessionIssuanceWindowSeconds)
	assert.Equal(t, 1, UserSessionRevokedRetentionDays)
	assert.Equal(t, 56, UserSessionHourlyAlertThreshold)

	t.Setenv("USER_SESSION_ISSUANCE_WINDOW_SECONDS", "43200")
	initUserSessionSettings()
	assert.Equal(t, int64(12*60*60), UserSessionIssuanceWindowSeconds, "a window below retention remains unchanged")

	t.Setenv("USER_SESSION_ISSUANCE_WINDOW_SECONDS", "86400")
	initUserSessionSettings()
	assert.Equal(t, int64(24*60*60), UserSessionIssuanceWindowSeconds, "a window equal to retention remains unchanged")

	t.Setenv("USER_SESSION_REVOKED_RETENTION_DAYS", "9223372036854775807")
	initUserSessionSettings()
	assert.Equal(t, DefaultUserSessionRevokedRetentionDays, UserSessionRevokedRetentionDays)
}

func TestInitEnvValidatesSessionConfiguration(t *testing.T) {
	validProductionSecret := "test-only-7M4qP2vN8xR5cK9wT3bH6jD1"
	tests := []struct {
		name           string
		secret         string
		secure         string
		trustedOrigin  string
		wantError      bool
		verifyFallback bool
	}{
		{name: "local development uses process fallback", verifyFallback: true},
		{name: "local development accepts a short explicit secret", secret: "local-secret"},
		{name: "secure production settings", secret: validProductionSecret, secure: "true", trustedOrigin: "https://yuaiapi.com,https://global.yuaiapi.com"},
		{name: "missing session secret", secure: "true", trustedOrigin: "https://yuaiapi.com", wantError: true},
		{name: "session secret shorter than 32 bytes", secret: "short-production-secret", secure: "true", trustedOrigin: "https://yuaiapi.com", wantError: true},
		{name: "placeholder session secret", secret: "random_string", secure: "true", trustedOrigin: "https://yuaiapi.com", wantError: true},
		{name: "repeated low-diversity secret", secret: strings.Repeat("s", 32), secure: "true", trustedOrigin: "https://yuaiapi.com", wantError: true},
		{name: "missing trusted origin", secret: validProductionSecret, secure: "true", wantError: true},
		{name: "insecure trusted origin", secret: validProductionSecret, secure: "true", trustedOrigin: "http://yuaiapi.com", wantError: true},
		{name: "trusted origin with path", secret: validProductionSecret, secure: "true", trustedOrigin: "https://yuaiapi.com/dashboard", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifyFallback := "0"
			if test.verifyFallback {
				verifyFallback = "1"
			}
			command := exec.Command(os.Args[0], "-test.run=^TestProductionSessionConfigurationHelper$")
			command.Env = append(os.Environ(),
				"GO_WANT_PRODUCTION_SESSION_CONFIG_HELPER=1",
				"GO_VERIFY_LOCAL_SESSION_SECRET_FALLBACK="+verifyFallback,
				"SESSION_SECRET="+test.secret,
				"SESSION_COOKIE_SECURE="+test.secure,
				"SESSION_COOKIE_TRUSTED_URL="+test.trustedOrigin,
			)

			err := command.Run()
			if test.wantError {
				require.Error(t, err, "invalid production session configuration must stop startup")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestProductionSessionConfigurationHelper(t *testing.T) {
	if os.Getenv("GO_WANT_PRODUCTION_SESSION_CONFIG_HELPER") != "1" {
		return
	}
	*LogDir = os.TempDir()
	verifyFallback := os.Getenv("GO_VERIFY_LOCAL_SESSION_SECRET_FALLBACK") == "1"
	var initialSessionSecret string
	if verifyFallback {
		initialSessionSecret = SessionSecret
		require.NotEmpty(t, initialSessionSecret)
	}
	InitEnv()
	if verifyFallback {
		assert.Equal(t, initialSessionSecret, SessionSecret)
	}
}
