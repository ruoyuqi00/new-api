package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/xai"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAdvancedCustomModelListChannel(baseURL string, key string, upstreamPath string, auth *dto.AdvancedCustomRouteAuth) *model.Channel {
	channel := &model.Channel{
		Type:    constant.ChannelTypeAdvancedCustom,
		Key:     key,
		BaseURL: &baseURL,
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{
			IncomingPath: dto.AdvancedCustomModelListPath,
			UpstreamPath: upstreamPath,
			Converter:    dto.AdvancedCustomConverterNone,
			Auth:         auth,
		}}},
	})
	return channel
}

func TestParseOpenAIModelIDsStrictResponseContract(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		want      []string
		wantError string
	}{
		{name: "malformed JSON", body: `{"data":`, wantError: "invalid OpenAI Models response"},
		{name: "missing data", body: `{"object":"list"}`, wantError: "data is required"},
		{name: "null data", body: `{"data":null}`, wantError: "data is required"},
		{name: "empty data", body: `{"data":[]}`, wantError: "no valid model IDs"},
		{name: "empty IDs", body: `{"data":[{"id":""},{"id":"  "}]}`, wantError: "no valid model IDs"},
		{name: "normalizes IDs", body: `{"data":[{"id":" gpt-4.1 "},{"id":"gpt-4.1"},{"id":"o3"}]}`, want: []string{"gpt-4.1", "o3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			models, err := parseOpenAIModelIDs([]byte(tt.body))
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				require.Nil(t, models)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, models)
		})
	}
}

func TestFetchAdvancedCustomModelsAppliesRouteAuthAndHeaderOverrides(t *testing.T) {
	type receivedRequest struct {
		Header http.Header
		Host   string
	}
	received := make(chan receivedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/provider/models", r.URL.Path)
		received <- receivedRequest{Header: r.Header.Clone(), Host: r.Host}
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4.1"}]}`))
	}))
	t.Cleanup(server.Close)

	channel := newAdvancedCustomModelListChannel(server.URL, "secret-key", "/provider/models", &dto.AdvancedCustomRouteAuth{
		Type:  dto.AdvancedCustomAuthTypeHeader,
		Name:  "X-Route-Key",
		Value: "route-{api_key}",
	})
	headerOverride := `{"X-Route-Key":"global-{api_key}","X-Static":"static-value","Host":"models.example.test","*":""}`
	channel.HeaderOverride = &headerOverride

	models, err := fetchChannelUpstreamModelIDs(channel)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-4.1"}, models)

	request := <-received
	require.Equal(t, "global-secret-key", request.Header.Get("X-Route-Key"))
	require.Equal(t, "static-value", request.Header.Get("X-Static"))
	require.Equal(t, "models.example.test", request.Host)
}

func TestFetchAdvancedCustomModelsRedactsQueryKeyFromTransportErrors(t *testing.T) {
	const secret = "secret key/+"
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := server.URL
	server.Close()

	channel := newAdvancedCustomModelListChannel(baseURL, secret, "/provider/models", &dto.AdvancedCustomRouteAuth{
		Type:  dto.AdvancedCustomAuthTypeQuery,
		Name:  "custom-token",
		Value: "prefix-{api_key}",
	})

	_, err := fetchChannelUpstreamModelIDs(channel)
	require.Error(t, err)
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), "custom-token")

	direct := sanitizeFetchModelsError(&url.Error{
		Op:  http.MethodGet,
		URL: baseURL + "/provider/models?custom-token=prefix-" + url.QueryEscape(secret),
		Err: errors.New("connection refused"),
	}, secret)
	require.EqualError(t, direct, "connection refused")
}

func TestFetchAdvancedCustomModelsUsesBoundAccountPoolMapping(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AccountPool{}, &model.ProviderAccount{}, &model.ChannelAccountPoolBinding{}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/provider/models", r.URL.Path)
		switch r.Header.Get("Authorization") {
		case "Bearer account-key-a":
			_, _ = w.Write([]byte(`{"data":[{"id":"upstream-private-a"}]}`))
		case "Bearer account-key-b":
			_, _ = w.Write([]byte(`{"data":[{"id":"upstream-private-b"}]}`))
		default:
			http.Error(w, "unexpected credential", http.StatusUnauthorized)
		}
	}))
	t.Cleanup(server.Close)

	channel := newAdvancedCustomModelListChannel(server.URL, "unused-channel-key", "/provider/models", nil)
	channel.Name = "private advanced custom"
	channel.Group = "private-group"
	channel.Models = "private-model"
	require.NoError(t, db.Create(channel).Error)

	pool := model.AccountPool{
		Name:        "private advanced pool",
		AdapterType: constant.ChannelTypeAdvancedCustom,
		Group:       "private-group",
		Status:      model.AccountPoolStatusEnabled,
	}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&[]model.ProviderAccount{
		{
			PoolId:       pool.Id,
			Name:         "private account a",
			Type:         "api_key",
			Credential:   "account-key-a",
			ModelMapping: `{"private-model":"upstream-private-a"}`,
			Status:       model.ProviderAccountEnabled,
		},
		{
			PoolId:       pool.Id,
			Name:         "private account b",
			Type:         "api_key",
			Credential:   "account-key-b",
			ModelMapping: `{"private-model":"upstream-private-b"}`,
			Status:       model.ProviderAccountEnabled,
		},
	}).Error)
	require.NoError(t, db.Create(&model.ChannelAccountPoolBinding{
		ChannelId: channel.Id,
		PoolId:    pool.Id,
		Enabled:   true,
	}).Error)

	models, err := fetchChannelUpstreamModelIDs(channel)
	require.NoError(t, err)
	require.Equal(t, []string{"private-model"}, models)

	reloaded, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, "private-group", reloaded.Group)
	require.Equal(t, "private-model", reloaded.Models)
}

func TestFailedAdvancedCustomDetectionDoesNotStageFullRemoval(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AccountPool{}, &model.ProviderAccount{}, &model.ChannelAccountPoolBinding{}))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/provider/models", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(server.Close)

	channel := newAdvancedCustomModelListChannel(server.URL, "secret-key", "/provider/models", nil)
	channel.Name = "empty advanced custom discovery"
	channel.Models = "gpt-4.1,o3"
	channel.Group = "private-group"
	settings := channel.GetOtherSettings()
	settings.UpstreamModelUpdateCheckEnabled = true
	settings.UpstreamModelUpdateAutoSyncEnabled = true
	channel.SetOtherSettings(settings)
	require.NoError(t, db.Create(channel).Error)

	changed, autoAdded, err := checkAndPersistChannelUpstreamModelUpdates(channel, &settings, true, true)
	require.ErrorContains(t, err, "no valid model IDs")
	require.False(t, changed)
	require.Zero(t, autoAdded)
	require.Empty(t, settings.UpstreamModelUpdateLastDetectedModels)
	require.Empty(t, settings.UpstreamModelUpdateLastRemovedModels)

	reloaded, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, "private-group", reloaded.Group)
	require.Equal(t, "gpt-4.1,o3", reloaded.Models)
}

func TestNormalizeModelNames(t *testing.T) {
	result := normalizeModelNames([]string{
		" gpt-4o ",
		"",
		"gpt-4o",
		"gpt-4.1",
		"   ",
	})

	require.Equal(t, []string{"gpt-4o", "gpt-4.1"}, result)
}

func TestMergeModelNames(t *testing.T) {
	result := mergeModelNames(
		[]string{"gpt-4o", "gpt-4.1"},
		[]string{"gpt-4.1", " gpt-4.1-mini ", "gpt-4o"},
	)

	require.Equal(t, []string{"gpt-4o", "gpt-4.1", "gpt-4.1-mini"}, result)
}

func TestSubtractModelNames(t *testing.T) {
	result := subtractModelNames(
		[]string{"gpt-4o", "gpt-4.1", "gpt-4.1-mini"},
		[]string{"gpt-4.1", "not-exists"},
	)

	require.Equal(t, []string{"gpt-4o", "gpt-4.1-mini"}, result)
}

func TestIntersectModelNames(t *testing.T) {
	result := intersectModelNames(
		[]string{"gpt-4o", "gpt-4.1", "gpt-4.1", "not-exists"},
		[]string{"gpt-4.1", "gpt-4o-mini", "gpt-4o"},
	)

	require.Equal(t, []string{"gpt-4o", "gpt-4.1"}, result)
}

func TestFetchChannelUpstreamModelIDsUsesBoundAccountPoolIntersection(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AccountPool{}, &model.ProviderAccount{}, &model.ChannelAccountPoolBinding{}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("Authorization") {
		case "Bearer key-a":
			_, _ = w.Write([]byte(`{"data":[{"id":"shared"},{"id":"upstream-a"}]}`))
		case "Bearer key-b":
			_, _ = w.Write([]byte(`{"data":[{"id":"shared"},{"id":"upstream-b"}]}`))
		default:
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
	}))
	t.Cleanup(server.Close)

	baseURL := server.URL
	channel := model.Channel{Name: "pooled", Type: 1, Key: "", BaseURL: &baseURL, Status: 1, Models: "shared", Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	pool := model.AccountPool{Name: "pool", AdapterType: 1, Group: "default", Status: model.AccountPoolStatusEnabled}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&[]model.ProviderAccount{
		{PoolId: pool.Id, Name: "account-a", Type: "api_key", Credential: "key-a", ModelMapping: `{"public-image":"upstream-a"}`, Status: model.ProviderAccountEnabled},
		{PoolId: pool.Id, Name: "account-b", Type: "api_key", Credential: "key-b", ModelMapping: `{"public-image":"upstream-b"}`, Status: model.ProviderAccountEnabled},
	}).Error)
	require.NoError(t, db.Create(&model.ChannelAccountPoolBinding{ChannelId: channel.Id, PoolId: pool.Id, Enabled: true}).Error)

	models, err := fetchChannelUpstreamModelIDs(&channel)
	require.NoError(t, err)
	require.Equal(t, []string{"public-image", "shared"}, models)
}

func TestFetchChannelUpstreamModelIDsUsesUnboundAdapterPoolIntersection(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AccountPool{}, &model.ProviderAccount{}, &model.ChannelAccountPoolBinding{}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("Authorization") {
		case "Bearer key-a":
			_, _ = w.Write([]byte(`{"data":[{"id":"shared"},{"id":"account-a"}]}`))
		case "Bearer key-b":
			_, _ = w.Write([]byte(`{"data":[{"id":"shared"},{"id":"account-b"}]}`))
		default:
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
	}))
	t.Cleanup(server.Close)

	baseURL := server.URL
	channel := model.Channel{Name: "automatic-pool", Type: 1, Key: "invalid-channel-key", BaseURL: &baseURL, Status: 1, Models: "shared", Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	pool := model.AccountPool{Name: "automatic", AdapterType: 1, Group: "default", Status: model.AccountPoolStatusEnabled}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&[]model.ProviderAccount{
		{PoolId: pool.Id, Name: "account-a", Type: "api_key", Credential: "key-a", Status: model.ProviderAccountEnabled},
		{PoolId: pool.Id, Name: "account-b", Type: "api_key", Credential: "key-b", Status: model.ProviderAccountEnabled},
	}).Error)

	models, err := fetchChannelUpstreamModelIDs(&channel)
	require.NoError(t, err)
	require.Equal(t, []string{"shared"}, models)
}

func TestFetchChannelUpstreamModelIDsRejectsPartialPoolDiscovery(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AccountPool{}, &model.ProviderAccount{}, &model.ChannelAccountPoolBinding{}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"shared"}]}`))
	}))
	t.Cleanup(server.Close)

	baseURL := server.URL
	channel := model.Channel{Name: "pooled", Type: 1, BaseURL: &baseURL, Status: 1, Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	pool := model.AccountPool{Name: "pool", AdapterType: 1, Group: "default", Status: model.AccountPoolStatusEnabled}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&[]model.ProviderAccount{
		{PoolId: pool.Id, Name: "valid", Type: "api_key", Credential: "valid-key", Status: model.ProviderAccountEnabled},
		{PoolId: pool.Id, Name: "invalid", Type: "api_key", Credential: "invalid-key", Status: model.ProviderAccountEnabled},
	}).Error)
	require.NoError(t, db.Create(&model.ChannelAccountPoolBinding{ChannelId: channel.Id, PoolId: pool.Id, Enabled: true}).Error)

	discovery, err := discoverAccountPoolModels(pool.Id, channel.Id)
	require.NoError(t, err)
	require.False(t, discovery.Complete)
	require.Equal(t, 1, discovery.SucceededAccounts)
	require.Equal(t, 1, discovery.FailedAccounts)
	require.Equal(t, []string{"shared"}, discovery.CommonModels)

	_, err = fetchChannelUpstreamModelIDs(&channel)
	require.ErrorContains(t, err, "仅成功探测 1/2")
}

func TestFetchCredentialUpstreamModelIDsUsesCodexAccountCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/backend-api/codex/models", r.URL.Path)
		require.Equal(t, "0.144.1", r.URL.Query().Get("client_version"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		require.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
		require.Equal(t, "account-123", r.Header.Get("chatgpt-account-id"))
		require.Equal(t, "codex_cli_rs", r.Header.Get("originator"))
		require.Equal(t, "codex_cli_rs/0.144.1 (Mac OS 26.3.1; arm64) iTerm.app/3.6.9", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"slug":"gpt-visible","visibility":"list"},{"slug":"gpt-hidden","visibility":"hide"},{"slug":"gpt-internal","visibility":"none"}]}`))
	}))
	t.Cleanup(server.Close)

	baseURL := server.URL
	channel := &model.Channel{
		Name:    "codex",
		Type:    constant.ChannelTypeCodex,
		Key:     `{"access_token":"access-token","account_id":"account-123"}`,
		BaseURL: &baseURL,
	}

	models, err := fetchCredentialUpstreamModelIDs(channel)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-visible"}, models)
}

func TestDiscoverAccountPoolModelsQuarantinesCodexOAuthManifest401(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AccountPool{}, &model.ProviderAccount{}, &model.ChannelAccountPoolBinding{}))

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/backend-api/codex/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("Authorization") {
		case "Bearer revoked-token":
			http.Error(w, `{"detail":"Token revoked"}`, http.StatusUnauthorized)
		case "Bearer healthy-token":
			_, _ = w.Write([]byte(`{"models":[{"slug":"gpt-visible","visibility":"list"}]}`))
		default:
			http.Error(w, "unexpected credential", http.StatusUnauthorized)
		}
	}))
	t.Cleanup(server.Close)

	baseURL := server.URL
	channel := model.Channel{Name: "codex-pool", Type: constant.ChannelTypeCodex, BaseURL: &baseURL, Status: 1, Models: "gpt-visible", Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	// Keep AdapterType unset to cover pools that inherit Codex behavior from an
	// explicit channel binding.
	pool := model.AccountPool{Name: "codex-pool", Group: "default", Status: model.AccountPoolStatusEnabled}
	require.NoError(t, db.Create(&pool).Error)
	revoked := model.ProviderAccount{
		PoolId: pool.Id, Name: "revoked", Type: "oauth_json",
		Credential: `{"access_token":"revoked-token","account_id":"revoked-account"}`,
		Status:     model.ProviderAccountEnabled, Priority: 100,
	}
	healthy := model.ProviderAccount{
		PoolId: pool.Id, Name: "healthy", Type: "oauth_json",
		Credential: `{"access_token":"healthy-token","account_id":"healthy-account"}`,
		Status:     model.ProviderAccountEnabled, Priority: 50,
	}
	require.NoError(t, db.Create(&revoked).Error)
	require.NoError(t, db.Create(&healthy).Error)
	require.NoError(t, db.Create(&model.ChannelAccountPoolBinding{ChannelId: channel.Id, PoolId: pool.Id, Enabled: true}).Error)
	model.InitAccountPoolCache()

	discovery, err := discoverAccountPoolModels(pool.Id, channel.Id)
	require.NoError(t, err)
	require.False(t, discovery.Complete)
	require.Equal(t, 1, discovery.SucceededAccounts)
	require.Equal(t, 1, discovery.FailedAccounts)

	stored, err := model.GetProviderAccountSummary(revoked.Id)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, stored.UsageUpstreamStatus)
	require.Equal(t, "invalid_credential", stored.UsageErrorCode)
	require.Positive(t, stored.UsageCheckedAt)
	require.Contains(t, stored.UsageLastError, "status code: 401")

	lease, bound, err := model.AcquireProviderAccount(channel.Id, constant.ChannelTypeCodex, "default")
	require.NoError(t, err)
	require.True(t, bound)
	require.NotNil(t, lease)
	require.Equal(t, healthy.Id, lease.AccountId)
	lease.Release()
}

func TestResolveAccountPoolModelChannelRejectsDifferentExplicitBinding(t *testing.T) {
	pool := &model.AccountPool{Id: 7, AdapterType: constant.ChannelTypeOpenAI}

	_, err := resolveAccountPoolModelChannel(pool, []int{8}, 9)

	require.ErrorContains(t, err, "渠道 #9 未绑定账号池 #7")
}

func TestProviderAccountModelChannelPreservesCodexOAuthCredential(t *testing.T) {
	credential := `{"access_token":"access-token","account_id":"account-123"}`
	channel, err := providerAccountModelChannel(
		&model.Channel{Type: constant.ChannelTypeCodex},
		&model.AccountPool{AdapterType: constant.ChannelTypeCodex},
		model.ProviderAccount{Name: "codex-account", Type: "oauth_json", Credential: credential},
	)

	require.NoError(t, err)
	require.Equal(t, credential, channel.Key)
}

func TestProviderAccountModelChannelUsesGrokCLIBaseForOAuth(t *testing.T) {
	credential := `{"access_token":"access-token","refresh_token":"refresh-token"}`
	channel, err := providerAccountModelChannel(
		&model.Channel{Type: constant.ChannelTypeXai},
		&model.AccountPool{AdapterType: constant.ChannelTypeXai},
		model.ProviderAccount{Name: "grok-account", Type: "oauth_json", Credential: credential},
	)

	require.NoError(t, err)
	require.Equal(t, credential, channel.Key)
	require.NotNil(t, channel.BaseURL)
	require.Equal(t, constant.GrokOAuthBaseURL, *channel.BaseURL)
}

func TestGrokOAuthModelDiscoveryProbeUsesKnownCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/responses", r.URL.Path)
		require.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
		require.Equal(t, "xai-grok-cli", r.Header.Get("X-XAI-Token-Auth"))
		require.Equal(t, constant.GrokCLIClientVersion, r.Header.Get("x-grok-client-version"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","output":[]}`))
	}))
	t.Cleanup(server.Close)

	baseURL := server.URL
	channel := &model.Channel{
		Type:    constant.ChannelTypeXai,
		Key:     `{"access_token":"access-token"}`,
		BaseURL: &baseURL,
	}
	models, err := fetchCredentialUpstreamModelIDs(channel)
	require.NoError(t, err)
	require.Equal(t, xai.ModelList, models)
}

func TestApplySelectedModelChanges(t *testing.T) {
	t.Run("add and remove together", func(t *testing.T) {
		result := applySelectedModelChanges(
			[]string{"gpt-4o", "gpt-4.1", "claude-3"},
			[]string{"gpt-4.1-mini"},
			[]string{"claude-3"},
		)

		require.Equal(t, []string{"gpt-4o", "gpt-4.1", "gpt-4.1-mini"}, result)
	})

	t.Run("add wins when conflict with remove", func(t *testing.T) {
		result := applySelectedModelChanges(
			[]string{"gpt-4o"},
			[]string{"gpt-4.1"},
			[]string{"gpt-4.1"},
		)

		require.Equal(t, []string{"gpt-4o", "gpt-4.1"}, result)
	})
}

func TestCollectPendingApplyUpstreamModelChanges(t *testing.T) {
	settings := dto.ChannelOtherSettings{
		UpstreamModelUpdateLastDetectedModels: []string{" gpt-4o ", "gpt-4o", "gpt-4.1"},
		UpstreamModelUpdateLastRemovedModels:  []string{" old-model ", "", "old-model"},
	}

	pendingAddModels, pendingRemoveModels := collectPendingApplyUpstreamModelChanges(settings)

	require.Equal(t, []string{"gpt-4o", "gpt-4.1"}, pendingAddModels)
	require.Equal(t, []string{"old-model"}, pendingRemoveModels)
}

func TestRefreshChannelRuntimeCachePreservesProxyClients(t *testing.T) {
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCacheEnabled })
	service.ResetProxyClientCache()
	t.Cleanup(service.ResetProxyClientCache)

	proxyURL := "http://model-refresh-proxy.example:8080"
	before, err := service.GetHttpClientWithProxy(proxyURL)
	require.NoError(t, err)

	refreshChannelRuntimeCache()

	after, err := service.GetHttpClientWithProxy(proxyURL)
	require.NoError(t, err)
	assert.Same(t, before, after)
}

func TestNormalizeChannelModelMapping(t *testing.T) {
	modelMapping := `{
		" alias-model ": " upstream-model ",
		"": "invalid",
		"invalid-target": ""
	}`
	channel := &model.Channel{
		ModelMapping: &modelMapping,
	}

	result := normalizeChannelModelMapping(channel)
	require.Equal(t, map[string]string{
		"alias-model": "upstream-model",
	}, result)
}

func TestCollectPendingUpstreamModelChangesFromModels_WithModelMapping(t *testing.T) {
	pendingAddModels, pendingRemoveModels := collectPendingUpstreamModelChangesFromModels(
		[]string{"alias-model", "gpt-4o", "stale-model"},
		[]string{"gpt-4o", "gpt-4.1", "mapped-target"},
		[]string{"gpt-4.1"},
		map[string]string{
			"alias-model": "mapped-target",
		},
	)

	require.Equal(t, []string{}, pendingAddModels)
	require.Equal(t, []string{"stale-model"}, pendingRemoveModels)
}

func TestCollectPendingUpstreamModelChangesFromModels_WithIgnoredRegexPatterns(t *testing.T) {
	pendingAddModels, pendingRemoveModels := collectPendingUpstreamModelChangesFromModels(
		[]string{"gpt-4o"},
		[]string{"gpt-4o", "claude-3-5-sonnet", "sora-video", "gpt-4.1"},
		[]string{"regex:^sora-.*$", "gpt-4.1"},
		nil,
	)

	require.Equal(t, []string{"claude-3-5-sonnet"}, pendingAddModels)
	require.Equal(t, []string{}, pendingRemoveModels)
}

func TestBuildUpstreamModelUpdateTaskNotificationContent_OmitOverflowDetails(t *testing.T) {
	channelSummaries := make([]upstreamModelUpdateChannelSummary, 0, 12)
	for i := 0; i < 12; i++ {
		channelSummaries = append(channelSummaries, upstreamModelUpdateChannelSummary{
			ChannelName: "channel-" + string(rune('A'+i)),
			AddCount:    i + 1,
			RemoveCount: i,
		})
	}

	content := buildUpstreamModelUpdateTaskNotificationContent(
		24,
		12,
		56,
		21,
		9,
		[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		channelSummaries,
		[]string{
			"gpt-4.1", "gpt-4.1-mini", "o3", "o4-mini", "gemini-2.5-pro", "claude-3.7-sonnet",
			"qwen-max", "deepseek-r1", "llama-3.3-70b", "mistral-large", "command-r-plus", "doubao-pro-32k",
			"hunyuan-large",
		},
		[]string{
			"gpt-3.5-turbo", "claude-2.1", "gemini-1.5-pro", "mixtral-8x7b", "qwen-plus", "glm-4",
			"yi-large", "moonshot-v1", "doubao-lite",
		},
	)

	require.Contains(t, content, "其余 4 个渠道已省略")
	require.Contains(t, content, "其余 1 个已省略")
	require.Contains(t, content, "失败渠道 ID（展示 10/12）")
	require.Contains(t, content, "其余 2 个已省略")
}

func TestShouldSendUpstreamModelUpdateNotification(t *testing.T) {
	channelUpstreamModelUpdateNotifyState.Lock()
	channelUpstreamModelUpdateNotifyState.lastNotifiedAt = 0
	channelUpstreamModelUpdateNotifyState.lastChangedChannels = 0
	channelUpstreamModelUpdateNotifyState.lastFailedChannels = 0
	channelUpstreamModelUpdateNotifyState.Unlock()

	baseTime := int64(2000000)

	require.True(t, shouldSendUpstreamModelUpdateNotification(baseTime, 6, 0))
	require.False(t, shouldSendUpstreamModelUpdateNotification(baseTime+3600, 6, 0))
	require.True(t, shouldSendUpstreamModelUpdateNotification(baseTime+3600, 7, 0))
	require.False(t, shouldSendUpstreamModelUpdateNotification(baseTime+7200, 7, 0))
	require.True(t, shouldSendUpstreamModelUpdateNotification(baseTime+8000, 0, 3))
	require.False(t, shouldSendUpstreamModelUpdateNotification(baseTime+9000, 0, 3))
	require.True(t, shouldSendUpstreamModelUpdateNotification(baseTime+10000, 0, 4))
	require.True(t, shouldSendUpstreamModelUpdateNotification(baseTime+90000, 7, 0))
	require.True(t, shouldSendUpstreamModelUpdateNotification(baseTime+90001, 0, 0))
}

func TestDetectAllChannelUpstreamModelUpdatesRejectsExistingActiveTask(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))

	existing, err := model.CreateSystemTask(model.SystemTaskTypeModelUpdate, nil, nil)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/upstream-models/detect-all", nil)

	DetectAllChannelUpstreamModelUpdates(ctx)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), existing.TaskID)
	require.Contains(t, recorder.Body.String(), "已有模型更新任务正在运行或等待中")
}
