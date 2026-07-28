package model

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProviderAccountRoutingHealthUsesPersistedUsage(t *testing.T) {
	const now int64 = 1000
	tests := []struct {
		name             string
		account          ProviderAccount
		blocked          bool
		unavailableUntil int64
	}{
		{
			name: "deactivated workspace",
			account: ProviderAccount{
				UsageCheckedAt: now, UsageUpstreamStatus: http.StatusPaymentRequired,
			},
			blocked: true,
		},
		{
			name: "active account",
			account: ProviderAccount{
				UsageCheckedAt: now, UsageUpstreamStatus: http.StatusOK,
				UsageSnapshot: `{"rate_limit":{"allowed":true,"limit_reached":false}}`,
			},
		},
		{
			name: "rate limit reset pending",
			account: ProviderAccount{
				UsageCheckedAt: now, UsageUpstreamStatus: http.StatusOK,
				UsageSnapshot: `{"rate_limit":{"allowed":false,"limit_reached":true,"primary_window":{"reset_at":1200,"used_percent":100}}}`,
			},
			blocked: true, unavailableUntil: 1200,
		},
		{
			name: "rate limit reset elapsed",
			account: ProviderAccount{
				UsageCheckedAt: now - 200, UsageUpstreamStatus: http.StatusOK,
				UsageSnapshot: `{"rate_limit":{"allowed":false,"limit_reached":true,"primary_window":{"reset_at":900,"used_percent":100}}}`,
			},
		},
		{
			name: "transient usage failure",
			account: ProviderAccount{
				UsageCheckedAt: now, UsageUpstreamStatus: http.StatusBadGateway,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			health, blocked := providerAccountRoutingHealth(test.account, now)
			assert.Equal(t, test.blocked, blocked)
			assert.Equal(t, test.blocked, health.blocked)
			assert.Equal(t, test.unavailableUntil, health.unavailableUntil)
		})
	}
}

func TestProviderAccountAllowsGrokMediaGenerationUsesObservedEligibility(t *testing.T) {
	eligible := true
	ineligible := false
	tests := []struct {
		name    string
		account ProviderAccount
		want    bool
	}{
		{name: "api keys remain eligible", account: ProviderAccount{Type: "api_key", UsageSnapshot: `{"billing":{"status_code":403}}`}, want: true},
		{name: "unknown oauth eligibility remains compatible", account: ProviderAccount{Type: "oauth_json"}, want: true},
		{name: "billing forbidden oauth is excluded", account: ProviderAccount{Type: "oauth_json", UsageSnapshot: `{"billing":{"status_code":403}}`}, want: false},
		{name: "weekly billing forbidden oauth is excluded", account: ProviderAccount{Type: "oauth_json", UsageSnapshot: `{"billing":{"weekly_status_code":403}}`}, want: false},
		{name: "successful billing oauth is eligible", account: ProviderAccount{Type: "oauth_json", UsageSnapshot: `{"billing":{"status_code":200}}`}, want: true},
		{name: "explicit enable overrides billing", account: ProviderAccount{Type: "oauth_json", Metadata: `{"grok_media_eligible":true}`, UsageSnapshot: `{"billing":{"status_code":403}}`}, want: eligible},
		{name: "explicit disable overrides unknown", account: ProviderAccount{Type: "oauth_json", Metadata: `{"grok_media_eligible":false}`}, want: ineligible},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, providerAccountAllowsGrokMediaGeneration(test.account))
		})
	}
}

func TestAcquireProviderAccountSkipsPersistentlyUnavailableHigherPriorityAccount(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ChannelAccountPoolBinding{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ProviderAccount{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AccountPool{}).Error)

	pool := AccountPool{Name: "usage-aware", Group: "pro", Status: AccountPoolStatusEnabled}
	require.NoError(t, DB.Create(&pool).Error)
	dead := ProviderAccount{
		PoolId: pool.Id, Name: "dead", Credential: "dead-key", Status: ProviderAccountEnabled,
		Priority: 100, UsageCheckedAt: common.GetTimestamp(), UsageUpstreamStatus: http.StatusPaymentRequired,
	}
	healthy := ProviderAccount{
		PoolId: pool.Id, Name: "healthy", Credential: "healthy-key", Status: ProviderAccountEnabled,
		Priority: 50,
	}
	require.NoError(t, DB.Create(&dead).Error)
	require.NoError(t, DB.Create(&healthy).Error)
	require.NoError(t, DB.Create(&ChannelAccountPoolBinding{ChannelId: 99105, PoolId: pool.Id, Enabled: true}).Error)

	lease, bound, err := AcquireProviderAccount(99105, 1, "pro")
	require.NoError(t, err)
	require.True(t, bound)
	require.NotNil(t, lease)
	assert.Equal(t, healthy.Id, lease.AccountId)
	lease.Release()
}

func TestAcquireProviderAccountSkipsAccountsFailedEarlierInRequest(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))

	pool := AccountPool{Name: "request-failover", Group: "pro", Status: AccountPoolStatusEnabled}
	require.NoError(t, DB.Create(&pool).Error)
	t.Cleanup(func() { _ = DeleteAccountPool(pool.Id) })
	primary := ProviderAccount{PoolId: pool.Id, Name: "primary", Credential: "primary-key", Status: ProviderAccountEnabled, Priority: 100}
	secondary := ProviderAccount{PoolId: pool.Id, Name: "secondary", Credential: "secondary-key", Status: ProviderAccountEnabled, Priority: 50}
	require.NoError(t, DB.Create(&primary).Error)
	require.NoError(t, DB.Create(&secondary).Error)
	require.NoError(t, DB.Create(&ChannelAccountPoolBinding{ChannelId: 99106, PoolId: pool.Id, Enabled: true}).Error)

	lease, bound, err := AcquireProviderAccountWithOptions(99106, 1, "pro", ProviderAccountSelectionOptions{
		SkipAccountIDs: map[int]struct{}{primary.Id: {}},
	})
	require.NoError(t, err)
	require.True(t, bound)
	require.NotNil(t, lease)
	assert.Equal(t, secondary.Id, lease.AccountId)
	lease.Release()
}

func TestAcquireProviderAccountHonorsBindingGroupAndConcurrency(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ChannelAccountPoolBinding{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ProviderAccount{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AccountPool{}).Error)

	pool := AccountPool{Name: "runtime-pool", Provider: "openai", Group: "pro", Status: AccountPoolStatusEnabled, Priority: 100}
	require.NoError(t, DB.Create(&pool).Error)
	account := ProviderAccount{PoolId: pool.Id, Name: "primary", Type: "api_key", Credential: "secret-key", Status: ProviderAccountEnabled, Priority: 100, ConcurrencyLimit: 1}
	require.NoError(t, DB.Create(&account).Error)
	require.NoError(t, DB.Create(&ChannelAccountPoolBinding{ChannelId: 99101, PoolId: pool.Id, Enabled: true}).Error)

	lease, bound, err := AcquireProviderAccount(99101, 1, "pro")
	require.NoError(t, err)
	require.True(t, bound)
	require.NotNil(t, lease)
	assert.Equal(t, account.Id, lease.AccountId)
	assert.Equal(t, "secret-key", lease.Credential)

	second, bound, err := AcquireProviderAccount(99101, 1, "pro")
	require.Error(t, err)
	require.True(t, bound)
	assert.Nil(t, second)

	lease.Release()
	third, bound, err := AcquireProviderAccount(99101, 1, "pro")
	require.NoError(t, err)
	require.True(t, bound)
	require.NotNil(t, third)
	third.Release()

	missing, bound, err := AcquireProviderAccount(99101, 1, "default")
	require.NoError(t, err)
	require.False(t, bound)
	assert.Nil(t, missing)
}

func TestAcquireProviderAccountFillsHigherPriorityAccountBeforeSpillover(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ChannelAccountPoolBinding{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ProviderAccount{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AccountPool{}).Error)

	pool := AccountPool{Name: "priority-fill", Provider: "openai", Group: "pro", Status: AccountPoolStatusEnabled}
	require.NoError(t, DB.Create(&pool).Error)
	primary := ProviderAccount{PoolId: pool.Id, Name: "primary", Credential: "primary-key", Status: ProviderAccountEnabled, Priority: 100, ConcurrencyLimit: 2}
	secondary := ProviderAccount{PoolId: pool.Id, Name: "secondary", Credential: "secondary-key", Status: ProviderAccountEnabled, Priority: 50, ConcurrencyLimit: 2}
	require.NoError(t, DB.Create(&primary).Error)
	require.NoError(t, DB.Create(&secondary).Error)
	require.NoError(t, DB.Create(&ChannelAccountPoolBinding{ChannelId: 99104, PoolId: pool.Id, Enabled: true}).Error)

	first, bound, err := AcquireProviderAccount(99104, 1, "pro")
	require.NoError(t, err)
	require.True(t, bound)
	require.NotNil(t, first)
	second, bound, err := AcquireProviderAccount(99104, 1, "pro")
	require.NoError(t, err)
	require.True(t, bound)
	require.NotNil(t, second)
	third, bound, err := AcquireProviderAccount(99104, 1, "pro")
	require.NoError(t, err)
	require.True(t, bound)
	require.NotNil(t, third)

	assert.Equal(t, primary.Id, first.AccountId)
	assert.Equal(t, primary.Id, second.AccountId)
	assert.Equal(t, secondary.Id, third.AccountId)
	first.Release()
	second.Release()
	third.Release()
}

func TestUpdateProviderAccountRoutingPersistsIndependentLimits(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}))
	pool := AccountPool{Name: "routing-settings", Group: "default", Status: AccountPoolStatusEnabled}
	require.NoError(t, DB.Create(&pool).Error)
	account := ProviderAccount{PoolId: pool.Id, Name: "editable", Credential: "editable-key", Status: ProviderAccountEnabled}
	require.NoError(t, DB.Create(&account).Error)

	require.NoError(t, UpdateProviderAccountRouting(account.Id, 120, 7, 9, 30))
	var updated ProviderAccount
	require.NoError(t, DB.First(&updated, "id = ?", account.Id).Error)
	assert.Equal(t, int64(120), updated.Priority)
	assert.Equal(t, uint(7), updated.Weight)
	assert.Equal(t, 9, updated.ConcurrencyLimit)
	assert.Equal(t, 30, updated.CooldownSeconds)
	require.NoError(t, UpdateProviderAccountRouting(account.Id, 120, 7, 9, 30))
	assert.Error(t, UpdateProviderAccountRouting(account.Id, 0, 0, -1, 0))
	assert.Error(t, UpdateProviderAccountRouting(account.Id+100000, 0, 0, 0, 0))
}

func TestAcquireProviderAccountFallsBackToLowerPoolPriority(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ChannelAccountPoolBinding{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ProviderAccount{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AccountPool{}).Error)

	high := AccountPool{Name: "high", Group: "pro", Status: AccountPoolStatusEnabled, Priority: 100}
	low := AccountPool{Name: "low", Group: "pro", Status: AccountPoolStatusEnabled, Priority: 50}
	require.NoError(t, DB.Create(&high).Error)
	require.NoError(t, DB.Create(&low).Error)
	highAccount := ProviderAccount{PoolId: high.Id, Name: "high-account", Credential: "high-key", Status: ProviderAccountEnabled, ConcurrencyLimit: 1}
	lowAccount := ProviderAccount{PoolId: low.Id, Name: "low-account", Credential: "low-key", Status: ProviderAccountEnabled, ConcurrencyLimit: 1}
	require.NoError(t, DB.Create(&highAccount).Error)
	require.NoError(t, DB.Create(&lowAccount).Error)
	require.NoError(t, DB.Create(&[]ChannelAccountPoolBinding{
		{ChannelId: 99102, PoolId: high.Id, Enabled: true},
		{ChannelId: 99102, PoolId: low.Id, Enabled: true},
	}).Error)

	first, bound, err := AcquireProviderAccount(99102, 1, "pro")
	require.NoError(t, err)
	require.True(t, bound)
	require.NotNil(t, first)
	assert.Equal(t, highAccount.Id, first.AccountId)

	second, bound, err := AcquireProviderAccount(99102, 1, "pro")
	require.NoError(t, err)
	require.True(t, bound)
	require.NotNil(t, second)
	assert.Equal(t, lowAccount.Id, second.AccountId)
	first.Release()
	second.Release()
}

func TestAcquireProviderAccountUsesUnboundAdapterPoolAndReturnsUpstreamRoute(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ChannelAccountPoolBinding{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ProviderAccount{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AccountPool{}).Error)

	pool := AccountPool{Name: "automatic", AdapterType: 14, Group: "pro", Status: AccountPoolStatusEnabled}
	require.NoError(t, DB.Create(&pool).Error)
	account := ProviderAccount{
		PoolId: pool.Id, Name: "anthropic-account", Credential: "secret-key",
		BaseURL: "https://upstream.example", ModelMapping: `{"claude":"claude-upstream"}`,
		Status: ProviderAccountEnabled,
	}
	require.NoError(t, DB.Create(&account).Error)

	lease, managed, err := AcquireProviderAccount(88001, 14, "pro")
	require.NoError(t, err)
	require.True(t, managed)
	require.NotNil(t, lease)
	assert.Equal(t, account.BaseURL, lease.BaseURL)
	assert.Equal(t, account.ModelMapping, lease.ModelMapping)
	lease.Release()

	lease, managed, err = AcquireProviderAccount(88001, 1, "pro")
	require.NoError(t, err)
	assert.False(t, managed)
	assert.Nil(t, lease)

	lease, managed, err = AcquireProviderAccount(88001, 14, "default")
	require.NoError(t, err)
	assert.False(t, managed)
	assert.Nil(t, lease)
}
