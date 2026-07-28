package system_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAPIAddress(t *testing.T) {
	originalAPIAddress := APIAddress
	originalServerAddress := ServerAddress
	t.Cleanup(func() {
		APIAddress = originalAPIAddress
		ServerAddress = originalServerAddress
	})

	ServerAddress = " https://site.example.com/ "
	APIAddress = ""
	require.Equal(t, "https://site.example.com", GetAPIAddress())

	APIAddress = " https://api.example.com/// "
	require.Equal(t, "https://api.example.com", GetAPIAddress())
}
