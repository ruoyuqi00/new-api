package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSaveAccountPoolReplacesAccountsAndBindings(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ChannelAccountPoolBinding{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ProviderAccount{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AccountPool{}).Error)

	pool := &AccountPool{Name: "pool", Provider: "openai", Group: "pro", Status: AccountPoolStatusEnabled}
	accounts := []ProviderAccount{
		{Name: "keep", Credential: "key-a", Status: ProviderAccountEnabled},
		{Name: "remove", Credential: "key-b", Status: ProviderAccountEnabled},
	}
	require.NoError(t, SaveAccountPool(pool, accounts, []int{101, 102}))
	require.Positive(t, pool.Id)

	_, storedAccounts, channelIds, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Len(t, storedAccounts, 2)
	assert.ElementsMatch(t, []int{101, 102}, channelIds)

	pool.Name = "updated"
	kept := storedAccounts[0]
	if kept.Name != "keep" {
		kept = storedAccounts[1]
	}
	kept.Name = "kept"
	kept.Credential = ""
	require.NoError(t, SaveAccountPool(pool, []ProviderAccount{kept}, []int{102}))

	updatedPool, updatedAccounts, updatedChannelIds, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	assert.Equal(t, "updated", updatedPool.Name)
	require.Len(t, updatedAccounts, 1)
	assert.Equal(t, "kept", updatedAccounts[0].Name)
	assert.Equal(t, "key-a", updatedAccounts[0].Credential)
	assert.Equal(t, []int{102}, updatedChannelIds)

	updatedAccounts[0].Credential = ""
	require.NoError(t, SaveAccountPool(updatedPool, updatedAccounts, updatedChannelIds))
}

func TestSaveAccountPoolValidatesAccountUpstreamConfiguration(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))

	pool := &AccountPool{Name: "validated", AdapterType: 1, Group: "default"}
	err := SaveAccountPool(pool, []ProviderAccount{{
		Name: "invalid-url", Credential: "secret", BaseURL: "ftp://invalid.example",
	}}, nil)
	require.ErrorContains(t, err, "valid HTTP or HTTPS URL")

	pool.Id = 0
	err = SaveAccountPool(pool, []ProviderAccount{{
		Name: "invalid-mapping", Credential: "secret", ModelMapping: `{"gpt-4o":1}`,
	}}, nil)
	require.ErrorContains(t, err, "JSON object with string values")

	pool.Id = 0
	err = SaveAccountPool(pool, []ProviderAccount{{
		Name: "invalid-oauth", Type: "oauth_json", Credential: "not-json",
	}}, nil)
	require.ErrorContains(t, err, "valid JSON object")

	pool.Id = 0
	err = SaveAccountPool(pool, []ProviderAccount{
		{Name: "duplicate-a", Credential: "same-secret"},
		{Name: "duplicate-b", Credential: "same-secret"},
	}, nil)
	require.ErrorContains(t, err, "duplicate account credential")
}

func TestSaveAccountPoolValidatesCodexOAuthContract(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))

	pool := &AccountPool{Name: "codex", AdapterType: constant.ChannelTypeCodex, Group: "default"}
	err := SaveAccountPool(pool, []ProviderAccount{{
		Name: "missing-account", Type: "oauth_json", Credential: `{"access_token":"token"}`,
	}}, nil)
	require.ErrorContains(t, err, "requires access_token and account_id")

	pool.Id = 0
	require.NoError(t, SaveAccountPool(pool, []ProviderAccount{{
		Name: "valid", Type: "oauth_json", Credential: `{"access_token":"token","account_id":"account"}`,
	}}, nil))
	require.Positive(t, pool.Id)

	_, accounts, _, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, "oauth_json", accounts[0].Type)
}

func TestProviderAccountManagementImportAssignAndStatus(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))

	sourcePool := &AccountPool{Name: "managed-source", AdapterType: 1, Group: "pro"}
	require.NoError(t, SaveAccountPool(sourcePool, []ProviderAccount{{
		Name: "source-existing", Credential: "managed-key-a",
	}}, nil))
	targetPool := &AccountPool{Name: "managed-target", AdapterType: 1, Group: "default"}
	require.NoError(t, SaveAccountPool(targetPool, []ProviderAccount{{
		Name: "target-existing", Credential: "managed-key-z",
	}}, nil))
	mismatchedPool := &AccountPool{Name: "managed-mismatch", AdapterType: 14, Group: "default"}
	require.NoError(t, SaveAccountPool(mismatchedPool, []ProviderAccount{{
		Name: "mismatch-existing", Credential: "managed-key-y",
	}}, nil))
	t.Cleanup(func() {
		_ = DeleteAccountPool(sourcePool.Id)
		_ = DeleteAccountPool(targetPool.Id)
		_ = DeleteAccountPool(mismatchedPool.Id)
	})

	require.NoError(t, ImportProviderAccounts(sourcePool.Id, []ProviderAccount{
		{Name: "imported-b", Credential: "managed-key-b", ConcurrencyLimit: 2},
		{Name: "imported-c", Credential: "managed-key-c", Status: ProviderAccountDisabled},
	}))

	accounts, total, err := ListProviderAccounts("managed-", sourcePool.Id, 0, 0, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, accounts, 3)
	assert.Equal(t, "managed-source", accounts[0].PoolName)
	assert.Equal(t, "pro", accounts[0].PoolGroup)

	var movingId int
	for _, account := range accounts {
		if account.Name == "imported-b" {
			movingId = account.Id
		}
	}
	require.Positive(t, movingId)
	require.ErrorContains(t, AssignProviderAccountsToPool([]int{movingId}, mismatchedPool.Id), "adapter type")
	require.NoError(t, AssignProviderAccountsToPool([]int{movingId, movingId}, targetPool.Id))
	require.NoError(t, UpdateProviderAccountsStatus([]int{movingId, movingId}, ProviderAccountDisabled))

	moved, err := GetProviderAccountSummary(movingId)
	require.NoError(t, err)
	assert.Equal(t, targetPool.Id, moved.PoolId)
	assert.Equal(t, ProviderAccountDisabled, moved.Status)

	err = ImportProviderAccounts(targetPool.Id, []ProviderAccount{{
		Name: "duplicate", Credential: "managed-key-z",
	}})
	require.ErrorContains(t, err, "duplicate account credential")
}

func TestDeleteProviderAccountsDeletesUniqueExistingAccounts(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))

	pool := &AccountPool{Name: "batch-delete", AdapterType: 1, Group: "default"}
	require.NoError(t, SaveAccountPool(pool, []ProviderAccount{
		{Name: "delete-a", Credential: "batch-delete-a"},
		{Name: "delete-b", Credential: "batch-delete-b"},
		{Name: "keep", Credential: "batch-delete-keep"},
	}, nil))
	t.Cleanup(func() { _ = DeleteAccountPool(pool.Id) })

	_, accounts, _, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Len(t, accounts, 3)
	accountIds := make(map[string]int, len(accounts))
	for _, account := range accounts {
		accountIds[account.Name] = account.Id
	}

	deleted, err := DeleteProviderAccounts([]int{
		accountIds["delete-a"], accountIds["delete-a"], accountIds["delete-b"], 0,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 2, deleted)

	_, remaining, _, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, "keep", remaining[0].Name)

	_, err = DeleteProviderAccounts(nil)
	require.ErrorContains(t, err, "account IDs are required")
}

func TestProviderAccountUsageHealthPreservesLastSuccessfulSnapshot(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))

	pool := &AccountPool{Name: "usage-health", AdapterType: constant.ChannelTypeCodex, Group: "default"}
	require.NoError(t, SaveAccountPool(pool, []ProviderAccount{{
		Name: "usage-account", Type: "oauth_json",
		Credential: `{"access_token":"token","account_id":"account"}`,
	}}, nil))
	t.Cleanup(func() { _ = DeleteAccountPool(pool.Id) })

	_, accounts, _, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	accountId := accounts[0].Id

	require.NoError(t, UpdateProviderAccountUsageHealth(
		accountId,
		`{"plan_type":"plus","rate_limit":{"primary_window":{"used_percent":25}}}`,
		100,
		200,
		"",
		"",
	))
	require.NoError(t, UpdateProviderAccountUsageHealth(
		accountId,
		"",
		200,
		429,
		"rate_limit_exceeded",
		"HTTP 429: account is rate limited by upstream",
	))

	account, err := GetProviderAccountSummary(accountId)
	require.NoError(t, err)
	assert.Equal(t, int64(100), account.UsageUpdatedAt)
	assert.Contains(t, account.UsageSnapshot, `"plan_type":"plus"`)
	assert.Equal(t, int64(200), account.UsageCheckedAt)
	assert.Equal(t, 429, account.UsageUpstreamStatus)
	assert.Equal(t, "rate_limit_exceeded", account.UsageErrorCode)
	assert.Equal(t, "HTTP 429: account is rate limited by upstream", account.UsageLastError)

	require.NoError(t, UpdateProviderAccountUsageHealth(accountId, `{"plan_type":"team"}`, 300, 200, "", ""))
	account, err = GetProviderAccountSummary(accountId)
	require.NoError(t, err)
	assert.Equal(t, int64(300), account.UsageUpdatedAt)
	assert.Equal(t, int64(300), account.UsageCheckedAt)
	assert.Empty(t, account.UsageErrorCode)
	assert.Empty(t, account.UsageLastError)
}
