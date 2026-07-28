package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnableHTTP2KeepAlive(t *testing.T) {
	transport := &http.Transport{ForceAttemptHTTP2: true}

	h2Transport, err := enableHTTP2KeepAlive(transport)

	require.NoError(t, err)
	require.NotNil(t, h2Transport)
	assert.Equal(t, http2ReadIdleTimeout, h2Transport.ReadIdleTimeout)
	assert.Equal(t, http2PingTimeout, h2Transport.PingTimeout)
	assert.NotNil(t, transport.TLSNextProto["h2"])
}

func TestHTTPClientsEnableHTTP2KeepAlive(t *testing.T) {
	ResetProxyClientCache()
	t.Cleanup(ResetProxyClientCache)

	InitHttpClient()
	defaultTransport, ok := GetHttpClient().Transport.(*http.Transport)
	require.True(t, ok)
	assert.NotNil(t, defaultTransport.TLSNextProto["h2"])

	for _, proxyURL := range []string{
		"http://127.0.0.1:8080",
		"socks5://127.0.0.1:1080",
	} {
		t.Run(proxyURL, func(t *testing.T) {
			client, err := NewProxyHttpClient(proxyURL)
			require.NoError(t, err)
			transport, ok := client.Transport.(*http.Transport)
			require.True(t, ok)
			assert.True(t, transport.ForceAttemptHTTP2)
			assert.NotNil(t, transport.TLSNextProto["h2"])
		})
	}
}

func TestProxyClientCacheCanonicalizationAndTargetedInvalidation(t *testing.T) {
	ResetProxyClientCache()
	t.Cleanup(ResetProxyClientCache)
	InitHttpClient()

	first, err := GetHttpClientWithProxy(" HTTP://proxy.example:8080/ ")
	require.NoError(t, err)
	same, err := GetHttpClientWithProxy("http://proxy.example:8080")
	require.NoError(t, err)
	assert.Same(t, first, same)

	InvalidateProxyClient("http://proxy.example:8080")
	replacement, err := GetHttpClientWithProxy("http://proxy.example:8080/")
	require.NoError(t, err)
	assert.NotSame(t, first, replacement)
}
