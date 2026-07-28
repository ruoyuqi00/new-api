package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProxyURLStrict(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty"},
		{name: "http", raw: " http://proxy.example:8080 ", want: "http://proxy.example:8080"},
		{name: "https", raw: "https://proxy.example:8443", want: "https://proxy.example:8443"},
		{name: "socks default port", raw: "SOCKS5://proxy.example", want: "socks5://proxy.example:1080"},
		{name: "unsupported scheme", raw: "ftp://proxy.example", wantErr: true},
		{name: "missing host", raw: "http:///proxy", wantErr: true},
		{name: "invalid port", raw: "http://proxy.example:99999", wantErr: true},
		{name: "query", raw: "http://proxy.example:8080?x=1", wantErr: true},
		{name: "fragment", raw: "http://proxy.example:8080#fragment", wantErr: true},
		{name: "path", raw: "http://proxy.example:8080/path", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := ParseProxyURLStrict(test.raw)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if test.want == "" {
				assert.Nil(t, parsed)
				return
			}
			require.NotNil(t, parsed)
			assert.Equal(t, test.want, parsed.String())
		})
	}
}

func TestParseProxyURLRuntimeStripsLegacySuffix(t *testing.T) {
	parsed, stripped, err := ParseProxyURLRuntime("socks5://proxy.example/path?x=1#fragment")

	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.True(t, stripped)
	assert.Equal(t, "socks5://proxy.example:1080", parsed.String())
}
