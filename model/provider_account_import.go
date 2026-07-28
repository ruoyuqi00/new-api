package model

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

type ProviderAccountImportResult struct {
	Total   int `json:"total"`
	Created int `json:"created"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
}

func ImportProviderAccountsWithResult(poolId int, accounts []ProviderAccount) (ProviderAccountImportResult, error) {
	result := ProviderAccountImportResult{Total: len(accounts)}
	if poolId <= 0 || len(accounts) == 0 {
		return result, errors.New("target pool and provider accounts are required")
	}

	now := common.GetTimestamp()
	err := DB.Transaction(func(tx *gorm.DB) error {
		var pool AccountPool
		if err := tx.First(&pool, "id = ?", poolId).Error; err != nil {
			return err
		}

		var existing []ProviderAccount
		if err := tx.Where("pool_id = ?", poolId).Find(&existing).Error; err != nil {
			return err
		}
		existingByIdentity := make(map[string][]ProviderAccount, len(existing))
		for _, account := range existing {
			for _, identity := range providerAccountImportIdentities(account, pool.AdapterType, true) {
				existingByIdentity[identity] = append(existingByIdentity[identity], account)
			}
		}

		seen := make(map[string]struct{}, len(accounts))
		for i := range accounts {
			account := &accounts[i]
			account.Id = 0
			account.PoolId = poolId
			if err := normalizeProviderAccountImport(account, pool.AdapterType, now); err != nil {
				return fmt.Errorf("account %d: %w", i+1, err)
			}

			identities := providerAccountImportIdentities(*account, pool.AdapterType, false)
			if len(identities) == 0 {
				return fmt.Errorf("account %d: account identity is required", i+1)
			}
			if _, duplicate := seen[identities[0]]; duplicate {
				result.Skipped++
				continue
			}
			seen[identities[0]] = struct{}{}

			stored, found := findProviderAccountImportMatch(existingByIdentity, identities, *account)
			if !found {
				account.CreatedTime = now
				account.UpdatedTime = now
				if err := tx.Create(account).Error; err != nil {
					return err
				}
				result.Created++
				for _, identity := range providerAccountImportIdentities(*account, pool.AdapterType, true) {
					existingByIdentity[identity] = append(existingByIdentity[identity], *account)
				}
				continue
			}
			if pool.AdapterType != constant.ChannelTypeCodex &&
				(pool.AdapterType != constant.ChannelTypeXai || stored.Type != "oauth_json" || account.Type != "oauth_json") {
				return fmt.Errorf("account %d: duplicate account credential in target pool", i+1)
			}

			var merged string
			var mergeErr error
			if pool.AdapterType == constant.ChannelTypeCodex {
				merged, mergeErr = mergeCodexProviderAccountCredential(stored.Credential, account.Credential)
			} else {
				merged, mergeErr = mergeGrokProviderAccountCredential(stored.Credential, account.Credential)
			}
			if mergeErr != nil {
				return fmt.Errorf("account %d: %w", i+1, mergeErr)
			}
			if pool.AdapterType == constant.ChannelTypeCodex && providerAccountRefreshToken(*account) == "" && providerAccountRefreshToken(stored) != "" {
				account.ExpiresAt = stored.ExpiresAt
			}
			account.Credential = merged
			updates := map[string]interface{}{
				"name": account.Name, "type": account.Type, "credential": account.Credential,
				"base_url": account.BaseURL, "model_mapping": account.ModelMapping,
				"status": account.Status, "priority": account.Priority, "weight": account.Weight,
				"concurrency_limit": account.ConcurrencyLimit, "cooldown_seconds": account.CooldownSeconds,
				"expires_at": account.ExpiresAt, "metadata": account.Metadata, "updated_time": now,
			}
			if err := tx.Model(&ProviderAccount{}).Where("id = ?", stored.Id).Updates(updates).Error; err != nil {
				return err
			}
			account.Id = stored.Id
			account.CreatedTime = stored.CreatedTime
			account.UpdatedTime = now
			result.Updated++
			for _, identity := range providerAccountImportIdentities(*account, pool.AdapterType, true) {
				existingByIdentity[identity] = append(existingByIdentity[identity], *account)
			}
		}
		return nil
	})
	return result, err
}

func normalizeProviderAccountImport(account *ProviderAccount, adapterType int, now int64) error {
	account.Name = strings.TrimSpace(account.Name)
	account.Type = strings.TrimSpace(account.Type)
	account.Credential = strings.TrimSpace(account.Credential)
	account.BaseURL = strings.TrimRight(strings.TrimSpace(account.BaseURL), "/")
	account.ModelMapping = strings.TrimSpace(account.ModelMapping)
	if account.Name == "" {
		return errors.New("account name is required")
	}
	if account.Credential == "" {
		return errors.New("new account credential is required")
	}
	if adapterType == constant.ChannelTypeCodex {
		if err := normalizeCodexProviderAccount(account); err != nil {
			return err
		}
	} else if adapterType == constant.ChannelTypeXai && (account.Type == "oauth_json" || strings.HasPrefix(account.Credential, "{")) {
		if err := normalizeGrokProviderAccount(account); err != nil {
			return err
		}
	} else if account.Type == "" {
		account.Type = "api_key"
	}
	if account.BaseURL != "" {
		parsed, err := url.Parse(account.BaseURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return errors.New("account base URL must be a valid HTTP or HTTPS URL")
		}
	}
	if account.ModelMapping != "" {
		var mapping map[string]string
		if err := common.UnmarshalJsonStr(account.ModelMapping, &mapping); err != nil {
			return errors.New("account model mapping must be a JSON object with string values")
		}
	}
	if account.Type == "oauth_json" {
		var credential map[string]interface{}
		if err := common.UnmarshalJsonStr(account.Credential, &credential); err != nil {
			return errors.New("OAuth credential must be a valid JSON object")
		}
	}
	if account.Status == 0 {
		account.Status = ProviderAccountEnabled
	}
	if account.Status != ProviderAccountEnabled && account.Status != ProviderAccountDisabled {
		return errors.New("invalid provider account status")
	}
	if account.ConcurrencyLimit < 0 || account.CooldownSeconds < 0 {
		return errors.New("account concurrency and cooldown must not be negative")
	}
	if account.ExpiresAt > 0 && account.ExpiresAt <= now {
		return errors.New("account credential has expired")
	}
	return nil
}

func normalizeGrokProviderAccount(account *ProviderAccount) error {
	var credential map[string]interface{}
	if err := common.UnmarshalJsonStr(account.Credential, &credential); err != nil {
		return errors.New("Grok OAuth credential must be a valid JSON object")
	}
	accessToken := firstProviderCredentialString(credential,
		[]string{"tokens", "access_token"}, []string{"tokens", "accessToken"},
		[]string{"access_token"}, []string{"accessToken"}, []string{"token"},
	)
	if accessToken == "" {
		return errors.New("Grok OAuth credential requires access_token")
	}
	refreshToken := firstProviderCredentialString(credential,
		[]string{"tokens", "refresh_token"}, []string{"tokens", "refreshToken"},
		[]string{"refresh_token"}, []string{"refreshToken"},
	)
	idToken := firstProviderCredentialString(credential,
		[]string{"tokens", "id_token"}, []string{"tokens", "idToken"},
		[]string{"id_token"}, []string{"idToken"},
	)
	clientId := firstProviderCredentialString(credential, []string{"client_id"}, []string{"clientId"})
	email := firstProviderCredentialString(credential, []string{"email"}, []string{"user", "email"})
	idClaims := decodeProviderAccountJWT(idToken)
	accessClaims := decodeProviderAccountJWT(accessToken)
	subject := firstProviderCredentialString(credential, []string{"subject"}, []string{"sub"})
	if subject == "" {
		subject = providerCredentialString(idClaims["sub"])
	}
	if subject == "" {
		subject = providerCredentialString(accessClaims["sub"])
	}
	if email == "" {
		email = providerCredentialString(idClaims["email"])
	}
	if email == "" {
		email = providerCredentialString(accessClaims["email"])
	}
	expiresAt := providerCredentialExpiry(credential, accessClaims)
	if expiresAt == 0 {
		expiresAt = providerCredentialExpiry(credential, idClaims)
	}
	for _, key := range []string{
		"tokens", "user", "accessToken", "refreshToken", "idToken", "clientId", "token", "sub",
	} {
		delete(credential, key)
	}
	credential["access_token"] = accessToken
	credential["type"] = "grok"
	if refreshToken != "" {
		credential["refresh_token"] = refreshToken
	}
	if idToken != "" {
		credential["id_token"] = idToken
	}
	if clientId != "" {
		credential["client_id"] = clientId
	}
	if email != "" {
		credential["email"] = email
	}
	if subject != "" {
		credential["subject"] = subject
	}
	if expiresAt > 0 {
		credential["expires_at"] = time.Unix(expiresAt, 0).UTC().Format(time.RFC3339)
		delete(credential, "expiresAt")
		account.ExpiresAt = expiresAt
	}
	encoded, err := common.Marshal(credential)
	if err != nil {
		return fmt.Errorf("encode Grok OAuth credential: %w", err)
	}
	account.Type = "oauth_json"
	account.Credential = string(encoded)
	if account.BaseURL == "" || strings.EqualFold(strings.TrimRight(account.BaseURL, "/"), "https://api.x.ai") || strings.EqualFold(strings.TrimRight(account.BaseURL, "/"), "https://api.x.ai/v1") {
		account.BaseURL = constant.GrokOAuthBaseURL
	}
	return nil
}

func normalizeCodexProviderAccount(account *ProviderAccount) error {
	credential := make(map[string]interface{})
	if strings.HasPrefix(account.Credential, "{") {
		if err := common.UnmarshalJsonStr(account.Credential, &credential); err != nil {
			return errors.New("Codex OAuth credential must be a valid JSON object or access token")
		}
	} else {
		credential["access_token"] = account.Credential
	}

	accessToken := firstProviderCredentialString(credential,
		[]string{"tokens", "access_token"}, []string{"tokens", "accessToken"},
		[]string{"access_token"}, []string{"accessToken"}, []string{"token"},
	)
	if accessToken == "" {
		return errors.New("Codex OAuth credential requires access_token")
	}
	claims := decodeProviderAccountJWT(accessToken)
	auth, _ := claims["https://api.openai.com/auth"].(map[string]interface{})
	accountId := firstProviderCredentialString(credential,
		[]string{"chatgpt_account_id"}, []string{"chatgptAccountId"},
		[]string{"account_id"}, []string{"accountId"}, []string{"account", "id"},
		[]string{"account", "account_id"}, []string{"account", "chatgpt_account_id"},
	)
	if accountId == "" {
		accountId = providerCredentialString(auth["chatgpt_account_id"])
	}
	if accountId == "" {
		return errors.New("Codex OAuth credential requires account_id")
	}

	refreshToken := firstProviderCredentialString(credential,
		[]string{"tokens", "refresh_token"}, []string{"tokens", "refreshToken"},
		[]string{"refresh_token"}, []string{"refreshToken"},
	)
	idToken := firstProviderCredentialString(credential,
		[]string{"tokens", "id_token"}, []string{"tokens", "idToken"},
		[]string{"id_token"}, []string{"idToken"},
	)
	userId := firstProviderCredentialString(credential,
		[]string{"chatgpt_user_id"}, []string{"chatgptUserId"},
		[]string{"user_id"}, []string{"userId"}, []string{"user", "id"},
	)
	if userId == "" {
		userId = providerCredentialString(auth["chatgpt_user_id"])
	}
	if userId == "" {
		userId = providerCredentialString(auth["user_id"])
	}
	if userId == "" {
		userId = providerCredentialString(claims["sub"])
	}
	email := firstProviderCredentialString(credential, []string{"email"}, []string{"user", "email"})
	if email == "" {
		email = providerCredentialString(claims["email"])
	}
	planType := firstProviderCredentialString(credential,
		[]string{"plan_type"}, []string{"planType"},
		[]string{"account", "plan_type"}, []string{"account", "planType"},
	)
	if planType == "" {
		planType = providerCredentialString(auth["chatgpt_plan_type"])
	}
	expiresAt := providerCredentialExpiry(credential, claims)

	for _, key := range []string{
		"tokens", "account", "user", "session_token", "sessionToken", "expires",
		"accessToken", "token", "accountId", "chatgpt_account_id", "chatgptAccountId",
		"refreshToken", "idToken", "user_id", "userId", "chatgptUserId", "planType",
	} {
		delete(credential, key)
	}
	credential["access_token"] = accessToken
	credential["account_id"] = accountId
	credential["type"] = "codex"
	if refreshToken != "" {
		credential["refresh_token"] = refreshToken
	}
	if idToken != "" {
		credential["id_token"] = idToken
	}
	if userId != "" {
		credential["chatgpt_user_id"] = userId
	}
	if email != "" {
		credential["email"] = email
	}
	if planType != "" {
		credential["plan_type"] = planType
	}

	if expiresAt > 0 {
		credential["expired"] = time.Unix(expiresAt, 0).UTC().Format(time.RFC3339)
		delete(credential, "expires_at")
		delete(credential, "expiresAt")
		if refreshToken == "" && account.ExpiresAt == 0 {
			account.ExpiresAt = expiresAt
		}
	}
	encoded, err := common.Marshal(credential)
	if err != nil {
		return fmt.Errorf("encode Codex OAuth credential: %w", err)
	}
	account.Type = "oauth_json"
	account.Credential = string(encoded)
	return nil
}

func providerAccountImportIdentities(account ProviderAccount, adapterType int, stored bool) []string {
	credential := strings.TrimSpace(account.Credential)
	if credential == "" {
		return nil
	}
	if adapterType == constant.ChannelTypeXai && account.Type == "oauth_json" {
		var parsed map[string]interface{}
		if common.UnmarshalJsonStr(credential, &parsed) != nil {
			return []string{"access:" + providerCredentialFingerprint(credential)}
		}
		identities := make([]string, 0, 4)
		subject := providerCredentialString(parsed["subject"])
		if subject == "" {
			subject = providerCredentialString(decodeProviderAccountJWT(providerCredentialString(parsed["id_token"]))["sub"])
		}
		if subject == "" {
			subject = providerCredentialString(decodeProviderAccountJWT(providerCredentialString(parsed["access_token"]))["sub"])
		}
		if subject != "" {
			identities = append(identities, "grok-subject:"+subject)
		}
		if email := strings.ToLower(providerCredentialString(parsed["email"])); email != "" {
			identities = append(identities, "grok-email:"+email)
		}
		accessToken := providerCredentialString(parsed["access_token"])
		refreshToken := providerCredentialString(parsed["refresh_token"])
		if len(identities) == 0 && refreshToken != "" {
			identities = append(identities, "refresh:"+providerCredentialFingerprint(refreshToken))
		}
		if stored || len(identities) == 0 {
			identities = append(identities, "access:"+providerCredentialFingerprint(accessToken))
		}
		return identities
	}
	if adapterType != constant.ChannelTypeCodex {
		return []string{"credential:" + credential}
	}
	var parsed map[string]interface{}
	if common.UnmarshalJsonStr(credential, &parsed) != nil {
		return []string{"access:" + providerCredentialFingerprint(credential)}
	}
	accessToken := providerCredentialString(parsed["access_token"])
	refreshToken := providerCredentialString(parsed["refresh_token"])
	if refreshToken == "" {
		return []string{"access:" + providerCredentialFingerprint(accessToken)}
	}
	identities := make([]string, 0, 3)
	if userId := providerCredentialString(parsed["chatgpt_user_id"]); userId != "" {
		identities = append(identities, "user:"+userId)
	}
	if accountId := providerCredentialString(parsed["account_id"]); accountId != "" {
		identities = append(identities, "account:"+accountId)
	}
	if len(identities) == 0 {
		if email := strings.ToLower(providerCredentialString(parsed["email"])); email != "" {
			identities = append(identities, "email:"+email)
		}
	}
	if stored || len(identities) == 0 {
		identities = append(identities, "access:"+providerCredentialFingerprint(accessToken))
	}
	return identities
}

func findProviderAccountImportMatch(index map[string][]ProviderAccount, identities []string, incoming ProviderAccount) (ProviderAccount, bool) {
	incomingUserId := codexProviderAccountUserId(incoming)
	for _, identity := range identities {
		for _, candidate := range index[identity] {
			if strings.HasPrefix(identity, "account:") && incomingUserId != "" {
				candidateUserId := codexProviderAccountUserId(candidate)
				if candidateUserId != "" && candidateUserId != incomingUserId {
					continue
				}
			}
			return candidate, true
		}
	}
	return ProviderAccount{}, false
}

func codexProviderAccountUserId(account ProviderAccount) string {
	var credential map[string]interface{}
	if common.UnmarshalJsonStr(account.Credential, &credential) != nil {
		return ""
	}
	return providerCredentialString(credential["chatgpt_user_id"])
}

func providerAccountRefreshToken(account ProviderAccount) string {
	var credential map[string]interface{}
	if common.UnmarshalJsonStr(account.Credential, &credential) != nil {
		return ""
	}
	return providerCredentialString(credential["refresh_token"])
}

func mergeCodexProviderAccountCredential(existingRaw string, incomingRaw string) (string, error) {
	var existing map[string]interface{}
	var incoming map[string]interface{}
	if err := common.UnmarshalJsonStr(existingRaw, &existing); err != nil {
		return "", errors.New("stored Codex OAuth credential is invalid")
	}
	if err := common.UnmarshalJsonStr(incomingRaw, &incoming); err != nil {
		return "", errors.New("incoming Codex OAuth credential is invalid")
	}
	if providerCredentialString(incoming["refresh_token"]) == "" {
		for _, key := range []string{"refresh_token", "client_id"} {
			if value, ok := existing[key]; ok && providerCredentialString(value) != "" {
				incoming[key] = value
			}
		}
	}
	encoded, err := common.Marshal(incoming)
	if err != nil {
		return "", fmt.Errorf("encode merged Codex OAuth credential: %w", err)
	}
	return string(encoded), nil
}

func mergeGrokProviderAccountCredential(existingRaw string, incomingRaw string) (string, error) {
	var existing map[string]interface{}
	var incoming map[string]interface{}
	if err := common.UnmarshalJsonStr(existingRaw, &existing); err != nil {
		return "", errors.New("stored Grok OAuth credential is invalid")
	}
	if err := common.UnmarshalJsonStr(incomingRaw, &incoming); err != nil {
		return "", errors.New("incoming Grok OAuth credential is invalid")
	}
	for _, key := range []string{"refresh_token", "client_id", "email", "subject"} {
		if providerCredentialString(incoming[key]) == "" {
			if value, ok := existing[key]; ok && providerCredentialString(value) != "" {
				incoming[key] = value
			}
		}
	}
	encoded, err := common.Marshal(incoming)
	if err != nil {
		return "", fmt.Errorf("encode merged Grok OAuth credential: %w", err)
	}
	return string(encoded), nil
}

func firstProviderCredentialString(record map[string]interface{}, paths ...[]string) string {
	for _, path := range paths {
		var value interface{} = record
		for _, key := range path {
			object, ok := value.(map[string]interface{})
			if !ok {
				value = nil
				break
			}
			value = object[key]
		}
		if text := providerCredentialString(value); text != "" {
			return text
		}
	}
	return ""
}

func providerCredentialString(value interface{}) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func providerCredentialExpiry(credential map[string]interface{}, claims map[string]interface{}) int64 {
	for _, value := range []interface{}{credential["expired"], credential["expires_at"], credential["expiresAt"], claims["exp"]} {
		switch typed := value.(type) {
		case float64:
			if typed > 0 {
				return int64(typed)
			}
		case string:
			if unix, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil && unix > 0 {
				if unix >= 1_000_000_000_000 {
					return unix / 1000
				}
				return unix
			}
			if parsed, err := time.Parse(time.RFC3339, typed); err == nil {
				return parsed.Unix()
			}
		}
	}
	return 0
}

func decodeProviderAccountJWT(token string) map[string]interface{} {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return map[string]interface{}{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.RawStdEncoding.DecodeString(parts[1])
	}
	if err != nil {
		return map[string]interface{}{}
	}
	var claims map[string]interface{}
	if common.Unmarshal(payload, &claims) != nil {
		return map[string]interface{}{}
	}
	return claims
}

func providerCredentialFingerprint(value string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.TrimSpace(value))))
}
