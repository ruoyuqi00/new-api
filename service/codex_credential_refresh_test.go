package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRefreshCodexChannelCredentialPreservesProxyClients(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	ResetProxyClientCache()
	t.Cleanup(func() {
		ResetProxyClientCache()
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
	})

	proxyURL := "http://codex-refresh-proxy.example:8080"
	settingBytes, err := common.Marshal(dto.ChannelSettings{Proxy: proxyURL})
	require.NoError(t, err)
	setting := string(settingBytes)
	keyBytes, err := common.Marshal(CodexOAuthKey{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		Type:         "codex",
	})
	require.NoError(t, err)
	channel := model.Channel{
		Type:    constant.ChannelTypeCodex,
		Name:    "codex refresh",
		Key:     string(keyBytes),
		Models:  "gpt-test",
		Group:   "default",
		Status:  common.ChannelStatusEnabled,
		Setting: &setting,
	}
	require.NoError(t, db.Create(&channel).Error)

	oauthClient, err := GetHttpClientWithProxy(proxyURL)
	require.NoError(t, err)
	oauthClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`,
			)),
			Header: make(http.Header),
		}, nil
	})
	unrelatedURL := "http://unrelated-proxy.example:8080"
	before, err := GetHttpClientWithProxy(unrelatedURL)
	require.NoError(t, err)

	_, _, err = RefreshCodexChannelCredential(
		context.Background(),
		channel.Id,
		CodexCredentialRefreshOptions{ResetCaches: true},
	)
	require.NoError(t, err)
	after, err := GetHttpClientWithProxy(unrelatedURL)
	require.NoError(t, err)
	assert.Same(t, before, after)
}
