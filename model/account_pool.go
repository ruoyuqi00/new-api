package model

import (
	"errors"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

const (
	AccountPoolStatusEnabled  = 1
	AccountPoolStatusDisabled = 2
	ProviderAccountEnabled    = 1
	ProviderAccountDisabled   = 2
)

type AccountPool struct {
	Id          int    `json:"id"`
	Name        string `json:"name" gorm:"type:varchar(128);not null;index"`
	Provider    string `json:"provider" gorm:"type:varchar(64);index"`
	AdapterType int    `json:"adapter_type" gorm:"default:0;index"`
	Group       string `json:"group" gorm:"type:varchar(255);default:'default';index"`
	Status      int    `json:"status" gorm:"default:1;index"`
	Priority    int64  `json:"priority" gorm:"bigint;default:0;index"`
	Weight      uint   `json:"weight" gorm:"default:0"`
	Remark      string `json:"remark" gorm:"type:varchar(255)"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

type ProviderAccount struct {
	Id                  int    `json:"id"`
	PoolId              int    `json:"pool_id" gorm:"not null;index"`
	Name                string `json:"name" gorm:"type:varchar(128);not null"`
	Type                string `json:"type" gorm:"type:varchar(32);default:'api_key'"`
	Credential          string `json:"-" gorm:"type:text;not null"`
	BaseURL             string `json:"base_url" gorm:"type:text"`
	ModelMapping        string `json:"model_mapping" gorm:"type:text"`
	Status              int    `json:"status" gorm:"default:1;index"`
	Priority            int64  `json:"priority" gorm:"bigint;default:0;index"`
	Weight              uint   `json:"weight" gorm:"default:0"`
	ConcurrencyLimit    int    `json:"concurrency_limit" gorm:"default:0"`
	CooldownSeconds     int    `json:"cooldown_seconds" gorm:"default:0"`
	ExpiresAt           int64  `json:"expires_at" gorm:"bigint;default:0;index"`
	LastUsedAt          int64  `json:"last_used_at" gorm:"bigint;default:0"`
	LastError           string `json:"last_error" gorm:"type:text"`
	Metadata            string `json:"metadata" gorm:"type:text"`
	UsageSnapshot       string `json:"-" gorm:"type:text"`
	UsageUpdatedAt      int64  `json:"usage_updated_at" gorm:"bigint;default:0"`
	UsageLastError      string `json:"usage_last_error" gorm:"type:text"`
	UsageErrorCode      string `json:"usage_error_code" gorm:"type:varchar(64)"`
	UsageUpstreamStatus int    `json:"usage_upstream_status" gorm:"default:0"`
	UsageCheckedAt      int64  `json:"usage_checked_at" gorm:"bigint;default:0;index"`
	CreatedTime         int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime         int64  `json:"updated_time" gorm:"bigint"`
}

type ChannelAccountPoolBinding struct {
	ChannelId int  `json:"channel_id" gorm:"primaryKey;autoIncrement:false"`
	PoolId    int  `json:"pool_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool `json:"enabled"`
}

type AccountPoolSummary struct {
	AccountPool
	AccountCount        int64 `json:"account_count"`
	EnabledAccountCount int64 `json:"enabled_account_count"`
	ChannelCount        int64 `json:"channel_count"`
}

type ProviderAccountSummary struct {
	ProviderAccount
	PoolName        string `json:"pool_name"`
	PoolGroup       string `json:"pool_group"`
	PoolAdapterType int    `json:"pool_adapter_type"`
	PoolStatus      int    `json:"pool_status"`
	ChannelCount    int64  `json:"channel_count"`
}

func normalizeProviderAccountIds(accountIds []int) []int {
	seen := make(map[int]struct{}, len(accountIds))
	normalized := make([]int, 0, len(accountIds))
	for _, accountId := range accountIds {
		if accountId <= 0 {
			continue
		}
		if _, exists := seen[accountId]; exists {
			continue
		}
		seen[accountId] = struct{}{}
		normalized = append(normalized, accountId)
	}
	return normalized
}

func ListProviderAccounts(keyword string, poolId int, status int, startIdx int, num int) ([]ProviderAccountSummary, int64, error) {
	query := DB.Table("provider_accounts").Joins("JOIN account_pools ON account_pools.id = provider_accounts.pool_id")
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("provider_accounts.name LIKE ? OR account_pools.name LIKE ? OR account_pools.provider LIKE ?", like, like, like)
	}
	if poolId > 0 {
		query = query.Where("provider_accounts.pool_id = ?", poolId)
	}
	if status > 0 {
		query = query.Where("provider_accounts.status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var accounts []ProviderAccountSummary
	selectFields := "provider_accounts.*, account_pools.name AS pool_name, account_pools." + commonGroupCol + " AS pool_group, account_pools.adapter_type AS pool_adapter_type, account_pools.status AS pool_status, (SELECT COUNT(*) FROM channel_account_pool_bindings WHERE channel_account_pool_bindings.pool_id = provider_accounts.pool_id AND channel_account_pool_bindings.enabled = ?) AS channel_count"
	if err := query.Select(selectFields, true).
		Order("provider_accounts.priority DESC, provider_accounts.id DESC").
		Limit(num).Offset(startIdx).Scan(&accounts).Error; err != nil {
		return nil, 0, err
	}
	return accounts, total, nil
}

func GetProviderAccountSummary(id int) (*ProviderAccountSummary, error) {
	var account ProviderAccountSummary
	selectFields := "provider_accounts.*, account_pools.name AS pool_name, account_pools." + commonGroupCol + " AS pool_group, account_pools.adapter_type AS pool_adapter_type, account_pools.status AS pool_status, (SELECT COUNT(*) FROM channel_account_pool_bindings WHERE channel_account_pool_bindings.pool_id = provider_accounts.pool_id AND channel_account_pool_bindings.enabled = ?) AS channel_count"
	err := DB.Table("provider_accounts").
		Select(selectFields, true).
		Joins("JOIN account_pools ON account_pools.id = provider_accounts.pool_id").
		Where("provider_accounts.id = ?", id).
		First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func GetProviderAccountSummaries(accountIds []int) ([]ProviderAccountSummary, error) {
	accountIds = normalizeProviderAccountIds(accountIds)
	if len(accountIds) == 0 {
		return []ProviderAccountSummary{}, nil
	}
	var accounts []ProviderAccountSummary
	selectFields := "provider_accounts.*, account_pools.name AS pool_name, account_pools." + commonGroupCol + " AS pool_group, account_pools.adapter_type AS pool_adapter_type, account_pools.status AS pool_status, (SELECT COUNT(*) FROM channel_account_pool_bindings WHERE channel_account_pool_bindings.pool_id = provider_accounts.pool_id AND channel_account_pool_bindings.enabled = ?) AS channel_count"
	err := DB.Table("provider_accounts").
		Select(selectFields, true).
		Joins("JOIN account_pools ON account_pools.id = provider_accounts.pool_id").
		Where("provider_accounts.id IN ?", accountIds).
		Scan(&accounts).Error
	return accounts, err
}

func AssignProviderAccountsToPool(accountIds []int, poolId int) error {
	accountIds = normalizeProviderAccountIds(accountIds)
	if poolId <= 0 || len(accountIds) == 0 {
		return errors.New("account IDs and target pool are required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var targetPool AccountPool
		if err := tx.First(&targetPool, "id = ?", poolId).Error; err != nil {
			return err
		}
		var moving []ProviderAccount
		if err := tx.Where("id IN ?", accountIds).Find(&moving).Error; err != nil {
			return err
		}
		if len(moving) != len(accountIds) {
			return errors.New("one or more provider accounts were not found")
		}
		sourcePoolIds := make([]int, 0, len(moving))
		for _, account := range moving {
			sourcePoolIds = append(sourcePoolIds, account.PoolId)
		}
		var sourcePools []AccountPool
		if err := tx.Where("id IN ?", sourcePoolIds).Find(&sourcePools).Error; err != nil {
			return err
		}
		sourceAdapterTypes := make(map[int]int, len(sourcePools))
		for _, pool := range sourcePools {
			sourceAdapterTypes[pool.Id] = pool.AdapterType
		}
		for _, account := range moving {
			sourceAdapterType := sourceAdapterTypes[account.PoolId]
			if targetPool.AdapterType > 0 && sourceAdapterType > 0 && targetPool.AdapterType != sourceAdapterType {
				return errors.New("provider account adapter type does not match target pool")
			}
		}
		var existing []string
		if err := tx.Model(&ProviderAccount{}).
			Where("pool_id = ? AND id NOT IN ?", poolId, accountIds).
			Pluck("credential", &existing).Error; err != nil {
			return err
		}
		credentials := make(map[string]struct{}, len(existing)+len(moving))
		for _, credential := range existing {
			credential = strings.TrimSpace(credential)
			if credential != "" {
				credentials[credential] = struct{}{}
			}
		}
		for _, account := range moving {
			credential := strings.TrimSpace(account.Credential)
			if _, duplicate := credentials[credential]; credential != "" && duplicate {
				return errors.New("duplicate account credential in target pool")
			}
			credentials[credential] = struct{}{}
		}
		return tx.Model(&ProviderAccount{}).Where("id IN ?", accountIds).Updates(map[string]interface{}{
			"pool_id":      poolId,
			"updated_time": common.GetTimestamp(),
		}).Error
	})
}

func UpdateProviderAccountsStatus(accountIds []int, status int) error {
	accountIds = normalizeProviderAccountIds(accountIds)
	if len(accountIds) == 0 {
		return errors.New("account IDs are required")
	}
	if status != ProviderAccountEnabled && status != ProviderAccountDisabled {
		return errors.New("invalid provider account status")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&ProviderAccount{}).Where("id IN ?", accountIds).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(accountIds)) {
			return errors.New("one or more provider accounts were not found")
		}
		return tx.Model(&ProviderAccount{}).Where("id IN ?", accountIds).Updates(map[string]interface{}{
			"status":       status,
			"updated_time": common.GetTimestamp(),
		}).Error
	})
}

func UpdateProviderAccountRouting(id int, priority int64, weight uint, concurrencyLimit int, cooldownSeconds int) error {
	if id <= 0 {
		return errors.New("provider account ID is required")
	}
	if concurrencyLimit < 0 || cooldownSeconds < 0 {
		return errors.New("account concurrency and cooldown must not be negative")
	}
	result := DB.Model(&ProviderAccount{}).Where("id = ?", id).Updates(map[string]interface{}{
		"priority":          priority,
		"weight":            weight,
		"concurrency_limit": concurrencyLimit,
		"cooldown_seconds":  cooldownSeconds,
		"updated_time":      common.GetTimestamp(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := DB.Model(&ProviderAccount{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.New("provider account not found")
		}
	}
	return nil
}

func ImportProviderAccounts(poolId int, accounts []ProviderAccount) error {
	_, err := ImportProviderAccountsWithResult(poolId, accounts)
	return err
}

func UpdateProviderAccountCredential(id int, credential string, expiresAt int64) error {
	updates := map[string]interface{}{
		"credential":   credential,
		"updated_time": common.GetTimestamp(),
	}
	if expiresAt > 0 {
		updates["expires_at"] = expiresAt
	}
	return DB.Model(&ProviderAccount{}).Where("id = ?", id).Updates(updates).Error
}

func UpdateProviderAccountUsageHealth(id int, snapshot string, checkedAt int64, upstreamStatus int, errorCode string, lastError string) error {
	errorCode = strings.TrimSpace(errorCode)
	if len(errorCode) > 64 {
		errorCode = errorCode[:64]
	}
	updates := map[string]interface{}{
		"usage_checked_at":      checkedAt,
		"usage_upstream_status": upstreamStatus,
		"usage_error_code":      errorCode,
		"usage_last_error":      strings.TrimSpace(lastError),
	}
	if strings.TrimSpace(snapshot) != "" {
		updates["usage_snapshot"] = snapshot
		updates["usage_updated_at"] = checkedAt
	}
	return DB.Model(&ProviderAccount{}).Where("id = ?", id).Updates(updates).Error
}

func ListAccountPools(keyword string, startIdx int, num int) ([]AccountPoolSummary, int64, error) {
	query := DB.Model(&AccountPool{})
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR provider LIKE ? OR "+commonGroupCol+" LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var summaries []AccountPoolSummary
	if err := query.Select(`account_pools.*,
        (SELECT COUNT(*) FROM provider_accounts WHERE provider_accounts.pool_id = account_pools.id) AS account_count,
        (SELECT COUNT(*) FROM provider_accounts WHERE provider_accounts.pool_id = account_pools.id AND provider_accounts.status = ?) AS enabled_account_count,
        (SELECT COUNT(*) FROM channel_account_pool_bindings WHERE channel_account_pool_bindings.pool_id = account_pools.id AND channel_account_pool_bindings.enabled = ?) AS channel_count`, ProviderAccountEnabled, true).
		Order("priority DESC, id DESC").Limit(num).Offset(startIdx).Scan(&summaries).Error; err != nil {
		return nil, 0, err
	}
	return summaries, total, nil
}

func GetAccountPoolDetail(id int) (*AccountPool, []ProviderAccount, []int, error) {
	var pool AccountPool
	if err := DB.First(&pool, "id = ?", id).Error; err != nil {
		return nil, nil, nil, err
	}
	var accounts []ProviderAccount
	if err := DB.Where("pool_id = ?", id).Order("priority DESC, id ASC").Find(&accounts).Error; err != nil {
		return nil, nil, nil, err
	}
	var channelIds []int
	if err := DB.Model(&ChannelAccountPoolBinding{}).Where("pool_id = ? AND enabled = ?", id, true).Pluck("channel_id", &channelIds).Error; err != nil {
		return nil, nil, nil, err
	}
	return &pool, accounts, channelIds, nil
}

func GetEnabledAccountPoolIdsByChannel(channelId int, adapterType int) ([]int, error) {
	if channelId <= 0 {
		return []int{}, nil
	}
	var poolIds []int
	query := DB.Model(&AccountPool{}).Where("account_pools.status = ?", AccountPoolStatusEnabled)
	if adapterType > 0 {
		query = query.Where(
			"EXISTS (SELECT 1 FROM channel_account_pool_bindings WHERE channel_account_pool_bindings.pool_id = account_pools.id AND channel_account_pool_bindings.channel_id = ? AND channel_account_pool_bindings.enabled = ?) OR (account_pools.adapter_type = ? AND NOT EXISTS (SELECT 1 FROM channel_account_pool_bindings WHERE channel_account_pool_bindings.pool_id = account_pools.id AND channel_account_pool_bindings.enabled = ?))",
			channelId,
			true,
			adapterType,
			true,
		)
	} else {
		query = query.Where(
			"EXISTS (SELECT 1 FROM channel_account_pool_bindings WHERE channel_account_pool_bindings.pool_id = account_pools.id AND channel_account_pool_bindings.channel_id = ? AND channel_account_pool_bindings.enabled = ?)",
			channelId,
			true,
		)
	}
	err := query.Order("account_pools.priority DESC, account_pools.id ASC").Pluck("account_pools.id", &poolIds).Error
	return poolIds, err
}

func SaveAccountPool(pool *AccountPool, accounts []ProviderAccount, channelIds []int) error {
	if pool == nil || strings.TrimSpace(pool.Name) == "" {
		return errors.New("account pool name is required")
	}
	if pool.Status != AccountPoolStatusDisabled && len(accounts) == 0 {
		return errors.New("enabled account pool requires at least one account")
	}
	if pool.AdapterType <= 0 && len(channelIds) == 0 {
		return errors.New("account pool requires an adapter type or channel binding")
	}
	now := common.GetTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		pool.Name = strings.TrimSpace(pool.Name)
		pool.Provider = strings.TrimSpace(pool.Provider)
		pool.Group = strings.TrimSpace(pool.Group)
		if pool.Group == "" {
			pool.Group = "default"
		}
		if pool.Status == 0 {
			pool.Status = AccountPoolStatusEnabled
		}
		pool.UpdatedTime = now
		if pool.Id == 0 {
			pool.CreatedTime = now
			if err := tx.Create(pool).Error; err != nil {
				return err
			}
		} else {
			var poolCount int64
			if err := tx.Model(&AccountPool{}).Where("id = ?", pool.Id).Count(&poolCount).Error; err != nil {
				return err
			}
			if poolCount == 0 {
				return errors.New("account pool not found")
			}
			if err := tx.Model(&AccountPool{}).Where("id = ?", pool.Id).Updates(map[string]interface{}{
				"name": pool.Name, "provider": pool.Provider, "adapter_type": pool.AdapterType, "group": pool.Group,
				"status": pool.Status, "priority": pool.Priority, "weight": pool.Weight,
				"remark": pool.Remark, "updated_time": now,
			}).Error; err != nil {
				return err
			}
		}
		if pool.Id > 0 {
			keptAccountIds := make([]int, 0, len(accounts))
			for _, account := range accounts {
				if account.Id > 0 {
					keptAccountIds = append(keptAccountIds, account.Id)
				}
			}
			deleteQuery := tx.Where("pool_id = ?", pool.Id)
			if len(keptAccountIds) > 0 {
				deleteQuery = deleteQuery.Where("id NOT IN ?", keptAccountIds)
			}
			if err := deleteQuery.Delete(&ProviderAccount{}).Error; err != nil {
				return err
			}
		}

		seenCredentials := make(map[string]struct{}, len(accounts))
		for i := range accounts {
			account := &accounts[i]
			account.PoolId = pool.Id
			account.Name = strings.TrimSpace(account.Name)
			account.Type = strings.TrimSpace(account.Type)
			if account.Name == "" {
				return errors.New("account name is required")
			}
			if account.Type == "" {
				account.Type = "api_key"
			}
			effectiveCredential := strings.TrimSpace(account.Credential)
			if effectiveCredential == "" && account.Id > 0 {
				var storedCredential string
				if err := tx.Model(&ProviderAccount{}).
					Select("credential").
					Where("id = ? AND pool_id = ?", account.Id, pool.Id).
					Scan(&storedCredential).Error; err != nil {
					return err
				}
				effectiveCredential = strings.TrimSpace(storedCredential)
			}
			if effectiveCredential != "" {
				if _, duplicate := seenCredentials[effectiveCredential]; duplicate {
					return errors.New("duplicate account credential in pool")
				}
				seenCredentials[effectiveCredential] = struct{}{}
			}
			account.BaseURL = strings.TrimRight(strings.TrimSpace(account.BaseURL), "/")
			account.ModelMapping = strings.TrimSpace(account.ModelMapping)
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
			if account.Type == "oauth_json" && effectiveCredential != "" {
				var credential map[string]interface{}
				if err := common.UnmarshalJsonStr(effectiveCredential, &credential); err != nil {
					return errors.New("OAuth credential must be a valid JSON object")
				}
				if pool.AdapterType == constant.ChannelTypeCodex {
					accessToken, _ := credential["access_token"].(string)
					accountId, _ := credential["account_id"].(string)
					if strings.TrimSpace(accessToken) == "" || strings.TrimSpace(accountId) == "" {
						return errors.New("Codex OAuth credential requires access_token and account_id")
					}
				}
			}
			if account.Status == 0 {
				account.Status = ProviderAccountEnabled
			}
			if account.ConcurrencyLimit < 0 || account.CooldownSeconds < 0 {
				return errors.New("account concurrency and cooldown must not be negative")
			}
			account.UpdatedTime = now
			if account.Id == 0 {
				if strings.TrimSpace(account.Credential) == "" {
					return errors.New("new account credential is required")
				}
				account.CreatedTime = now
				if err := tx.Create(account).Error; err != nil {
					return err
				}
				continue
			}
			updates := map[string]interface{}{
				"name": account.Name, "type": account.Type, "status": account.Status,
				"base_url": account.BaseURL, "model_mapping": account.ModelMapping,
				"priority": account.Priority, "weight": account.Weight,
				"concurrency_limit": account.ConcurrencyLimit, "cooldown_seconds": account.CooldownSeconds,
				"expires_at": account.ExpiresAt, "metadata": account.Metadata, "updated_time": now,
			}
			if strings.TrimSpace(account.Credential) != "" {
				updates["credential"] = account.Credential
			}
			var accountCount int64
			if err := tx.Model(&ProviderAccount{}).Where("id = ? AND pool_id = ?", account.Id, pool.Id).Count(&accountCount).Error; err != nil {
				return err
			}
			if accountCount == 0 {
				return errors.New("provider account does not belong to this pool")
			}
			if err := tx.Model(&ProviderAccount{}).Where("id = ? AND pool_id = ?", account.Id, pool.Id).Updates(updates).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("pool_id = ?", pool.Id).Delete(&ChannelAccountPoolBinding{}).Error; err != nil {
			return err
		}
		seen := make(map[int]struct{}, len(channelIds))
		for _, channelId := range channelIds {
			if channelId <= 0 {
				continue
			}
			if _, ok := seen[channelId]; ok {
				continue
			}
			seen[channelId] = struct{}{}
			if err := tx.Create(&ChannelAccountPoolBinding{ChannelId: channelId, PoolId: pool.Id, Enabled: true}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func DeleteAccountPool(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("pool_id = ?", id).Delete(&ChannelAccountPoolBinding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pool_id = ?", id).Delete(&ProviderAccount{}).Error; err != nil {
			return err
		}
		return tx.Delete(&AccountPool{}, "id = ?", id).Error
	})
}

func DeleteProviderAccount(id int) error {
	return DB.Delete(&ProviderAccount{}, "id = ?", id).Error
}

func DeleteProviderAccounts(accountIds []int) (int64, error) {
	accountIds = normalizeProviderAccountIds(accountIds)
	if len(accountIds) == 0 {
		return 0, errors.New("account IDs are required")
	}

	var deleted int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		const chunkSize = 200
		for start := 0; start < len(accountIds); start += chunkSize {
			end := min(start+chunkSize, len(accountIds))
			result := tx.Where("id IN ?", accountIds[start:end]).Delete(&ProviderAccount{})
			if result.Error != nil {
				return result.Error
			}
			deleted += result.RowsAffected
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

func DisableProviderAccount(id int, reason string) error {
	err := DB.Model(&ProviderAccount{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       ProviderAccountDisabled,
		"last_error":   reason,
		"updated_time": common.GetTimestamp(),
	}).Error
	if err == nil {
		InitAccountPoolCache()
	}
	return err
}
