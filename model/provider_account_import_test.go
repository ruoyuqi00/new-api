package model

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportProviderAccountsNormalizesRawCodexAccessToken(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}))
	pool := AccountPool{Name: "codex-raw-import", AdapterType: constant.ChannelTypeCodex, Group: "default", Status: AccountPoolStatusEnabled}
	require.NoError(t, DB.Create(&pool).Error)
	t.Cleanup(func() { _ = DeleteAccountPool(pool.Id) })

	expiresAt := time.Now().Add(24 * time.Hour).Unix()
	accessToken := providerAccountImportTestJWT(t, "account-raw", "user-raw", "raw@example.com", expiresAt)
	result, err := ImportProviderAccountsWithResult(pool.Id, []ProviderAccount{{
		Name: "raw", Type: "api_key", Credential: accessToken,
	}})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Zero(t, result.Updated)

	_, accounts, _, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, "oauth_json", accounts[0].Type)
	assert.Equal(t, expiresAt, accounts[0].ExpiresAt)
	var credential map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(accounts[0].Credential, &credential))
	assert.Equal(t, accessToken, credential["access_token"])
	assert.Equal(t, "account-raw", credential["account_id"])
	assert.Equal(t, "user-raw", credential["chatgpt_user_id"])
	assert.Equal(t, "raw@example.com", credential["email"])
}

func TestImportProviderAccountsUpdatesStableCodexIdentityAndPreservesRefreshToken(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))
	pool := AccountPool{Name: "codex-update-import", AdapterType: constant.ChannelTypeCodex, Group: "default", Status: AccountPoolStatusEnabled}
	require.NoError(t, DB.Create(&pool).Error)
	t.Cleanup(func() { _ = DeleteAccountPool(pool.Id) })

	expiresAt := time.Now().Add(24 * time.Hour).Unix()
	accessToken := providerAccountImportTestJWT(t, "account-update", "user-update", "update@example.com", expiresAt)
	initial := fmt.Sprintf(`{"access_token":%q,"refresh_token":"refresh-old","account_id":"account-update","chatgpt_user_id":"user-update"}`, accessToken)
	first, err := ImportProviderAccountsWithResult(pool.Id, []ProviderAccount{{Name: "initial", Credential: initial}})
	require.NoError(t, err)
	assert.Equal(t, 1, first.Created)
	require.NoError(t, DB.Model(&ProviderAccount{}).Where("pool_id = ?", pool.Id).Updates(map[string]interface{}{
		"status": ProviderAccountDisabled, "priority": int64(900), "weight": uint(77),
		"concurrency_limit": 4, "cooldown_seconds": 45,
		"base_url": "https://operator.example/v1", "model_mapping": `{"gpt-source":"gpt-target"}`,
	}).Error)

	second, err := ImportProviderAccountsWithResult(pool.Id, []ProviderAccount{{
		Name: "updated", Credential: accessToken, Metadata: `{"source":"refresh"}`,
		Status: ProviderAccountEnabled, Priority: 1, Weight: 1,
		ConcurrencyLimit: 1, CooldownSeconds: 1,
		BaseURL: "https://import.example/v1", ModelMapping: `{"other":"model"}`,
	}})
	require.NoError(t, err)
	assert.Zero(t, second.Created)
	assert.Equal(t, 1, second.Updated)

	_, accounts, _, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, "updated", accounts[0].Name)
	assert.Zero(t, accounts[0].ExpiresAt)
	assert.Equal(t, `{"source":"refresh"}`, accounts[0].Metadata)
	assert.Equal(t, ProviderAccountDisabled, accounts[0].Status)
	assert.Equal(t, int64(900), accounts[0].Priority)
	assert.Equal(t, uint(77), accounts[0].Weight)
	assert.Equal(t, 4, accounts[0].ConcurrencyLimit)
	assert.Equal(t, 45, accounts[0].CooldownSeconds)
	assert.Equal(t, "https://operator.example/v1", accounts[0].BaseURL)
	assert.Equal(t, `{"gpt-source":"gpt-target"}`, accounts[0].ModelMapping)
	var credential map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(accounts[0].Credential, &credential))
	assert.Equal(t, "refresh-old", credential["refresh_token"])
}

func TestImportProviderAccountsCreatesAccountWithImportedRouting(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}, &ChannelAccountPoolBinding{}))
	pool := AccountPool{Name: "codex-created-routing", AdapterType: constant.ChannelTypeCodex, Group: "default", Status: AccountPoolStatusEnabled}
	require.NoError(t, DB.Create(&pool).Error)
	t.Cleanup(func() { _ = DeleteAccountPool(pool.Id) })

	expiresAt := time.Now().Add(24 * time.Hour).Unix()
	accessToken := providerAccountImportTestJWT(t, "account-created", "user-created", "created@example.com", expiresAt)
	result, err := ImportProviderAccountsWithResult(pool.Id, []ProviderAccount{{
		Name: "created", Credential: accessToken,
		Status: ProviderAccountDisabled, Priority: 800, Weight: 63,
		ConcurrencyLimit: 6, CooldownSeconds: 30,
		BaseURL: "https://created.example/v1", ModelMapping: `{"gpt-source":"gpt-target"}`,
	}})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Zero(t, result.Updated)

	_, accounts, _, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, ProviderAccountDisabled, accounts[0].Status)
	assert.Equal(t, int64(800), accounts[0].Priority)
	assert.Equal(t, uint(63), accounts[0].Weight)
	assert.Equal(t, 6, accounts[0].ConcurrencyLimit)
	assert.Equal(t, 30, accounts[0].CooldownSeconds)
	assert.Equal(t, "https://created.example/v1", accounts[0].BaseURL)
	assert.Equal(t, `{"gpt-source":"gpt-target"}`, accounts[0].ModelMapping)
}

func TestImportProviderAccountsSkipsDuplicateCodexIdentityInSameBatch(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}))
	pool := AccountPool{Name: "codex-batch-duplicate", AdapterType: constant.ChannelTypeCodex, Group: "default", Status: AccountPoolStatusEnabled}
	require.NoError(t, DB.Create(&pool).Error)
	t.Cleanup(func() { _ = DeleteAccountPool(pool.Id) })

	expiresAt := time.Now().Add(24 * time.Hour).Unix()
	accessToken := providerAccountImportTestJWT(t, "account-batch", "user-batch", "batch@example.com", expiresAt)
	credential := fmt.Sprintf(`{"access_token":%q,"refresh_token":"refresh-batch","account_id":"account-batch","chatgpt_user_id":"user-batch"}`, accessToken)
	result, err := ImportProviderAccountsWithResult(pool.Id, []ProviderAccount{
		{Name: "first", Credential: credential},
		{Name: "duplicate", Credential: credential},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 1, result.Skipped)

	_, accounts, _, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	assert.Len(t, accounts, 1)
}

func TestImportProviderAccountsUpdatesStableGrokIdentityAndPreservesRefreshToken(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}))
	pool := AccountPool{Name: "grok-update-import", AdapterType: constant.ChannelTypeXai, Group: "default", Status: AccountPoolStatusEnabled}
	require.NoError(t, DB.Create(&pool).Error)
	t.Cleanup(func() { _ = DeleteAccountPool(pool.Id) })

	expiresAt := time.Now().Add(time.Hour).Unix()
	idToken := providerAccountImportGrokJWT(t, "grok-user", "grok@example.com", expiresAt)
	initial := fmt.Sprintf(`{"access_token":"access-old","refresh_token":"refresh-old","id_token":%q,"email":"grok@example.com","expires_at":%q}`, idToken, time.Unix(expiresAt, 0).UTC().Format(time.RFC3339))
	first, err := ImportProviderAccountsWithResult(pool.Id, []ProviderAccount{{Name: "initial", Type: "oauth_json", Credential: initial}})
	require.NoError(t, err)
	assert.Equal(t, 1, first.Created)

	updatedExpiry := time.Now().Add(2 * time.Hour).Unix()
	updated := fmt.Sprintf(`{"access_token":"access-new","id_token":%q,"email":"grok@example.com","expires_at":%q}`, idToken, time.Unix(updatedExpiry, 0).UTC().Format(time.RFC3339))
	second, err := ImportProviderAccountsWithResult(pool.Id, []ProviderAccount{{Name: "updated", Type: "oauth_json", Credential: updated}})
	require.NoError(t, err)
	assert.Zero(t, second.Created)
	assert.Equal(t, 1, second.Updated)

	_, accounts, _, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, "updated", accounts[0].Name)
	assert.Equal(t, updatedExpiry, accounts[0].ExpiresAt)
	var credential map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(accounts[0].Credential, &credential))
	assert.Equal(t, "access-new", credential["access_token"])
	assert.Equal(t, "refresh-old", credential["refresh_token"])
}

func TestImportProviderAccountsUsesGrokAccessTokenSubjectWithoutIDToken(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&AccountPool{}, &ProviderAccount{}))
	pool := AccountPool{Name: "grok-access-subject", AdapterType: constant.ChannelTypeXai, Group: "default", Status: AccountPoolStatusEnabled}
	require.NoError(t, DB.Create(&pool).Error)
	t.Cleanup(func() { _ = DeleteAccountPool(pool.Id) })

	expiresAt := time.Now().Add(time.Hour).Unix()
	firstToken := providerAccountImportGrokJWT(t, "grok-access-user", "grok@example.com", expiresAt)
	first, err := ImportProviderAccountsWithResult(pool.Id, []ProviderAccount{{Name: "first", Type: "oauth_json", Credential: fmt.Sprintf(`{"access_token":%q,"refresh_token":"refresh-old"}`, firstToken)}})
	require.NoError(t, err)
	assert.Equal(t, 1, first.Created)

	secondToken := providerAccountImportGrokJWT(t, "grok-access-user", "grok@example.com", expiresAt+3600)
	second, err := ImportProviderAccountsWithResult(pool.Id, []ProviderAccount{{Name: "second", Type: "oauth_json", Credential: fmt.Sprintf(`{"access_token":%q}`, secondToken)}})
	require.NoError(t, err)
	assert.Equal(t, 1, second.Updated)

	_, accounts, _, err := GetAccountPoolDetail(pool.Id)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	var credential map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(accounts[0].Credential, &credential))
	assert.Equal(t, "grok-access-user", credential["subject"])
	assert.Equal(t, "refresh-old", credential["refresh_token"])
}

func providerAccountImportTestJWT(t *testing.T, accountId string, userId string, email string, expiresAt int64) string {
	t.Helper()
	payload, err := common.Marshal(map[string]interface{}{
		"email": email,
		"exp":   expiresAt,
		"https://api.openai.com/auth": map[string]interface{}{
			"chatgpt_account_id": accountId,
			"chatgpt_user_id":    userId,
		},
	})
	require.NoError(t, err)
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func providerAccountImportGrokJWT(t *testing.T, subject string, email string, expiresAt int64) string {
	t.Helper()
	payload, err := common.Marshal(map[string]interface{}{
		"sub": subject, "email": email, "exp": expiresAt,
	})
	require.NoError(t, err)
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}
