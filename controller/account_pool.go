package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

type providerAccountInput struct {
	Id               int    `json:"id"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	Credential       string `json:"credential"`
	BaseURL          string `json:"base_url"`
	ModelMapping     string `json:"model_mapping"`
	Status           int    `json:"status"`
	Priority         int64  `json:"priority"`
	Weight           uint   `json:"weight"`
	ConcurrencyLimit int    `json:"concurrency_limit"`
	CooldownSeconds  int    `json:"cooldown_seconds"`
	ExpiresAt        int64  `json:"expires_at"`
	Metadata         string `json:"metadata"`
	AdapterType      int    `json:"adapter_type"`
}

type accountPoolInput struct {
	Id          int                    `json:"id"`
	Name        string                 `json:"name"`
	Provider    string                 `json:"provider"`
	AdapterType int                    `json:"adapter_type"`
	Group       string                 `json:"group"`
	Status      int                    `json:"status"`
	Priority    int64                  `json:"priority"`
	Weight      uint                   `json:"weight"`
	Remark      string                 `json:"remark"`
	Accounts    []providerAccountInput `json:"accounts"`
	ChannelIds  []int                  `json:"channel_ids"`
}

type providerAccountView struct {
	Id                  int      `json:"id"`
	PoolId              int      `json:"pool_id"`
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	Status              int      `json:"status"`
	Priority            int64    `json:"priority"`
	Weight              uint     `json:"weight"`
	ConcurrencyLimit    int      `json:"concurrency_limit"`
	CooldownSeconds     int      `json:"cooldown_seconds"`
	ExpiresAt           int64    `json:"expires_at"`
	LastUsedAt          int64    `json:"last_used_at"`
	LastError           string   `json:"last_error"`
	Metadata            string   `json:"metadata"`
	PlanType            string   `json:"plan_type"`
	PrimaryUsage        *float64 `json:"primary_usage_percent"`
	SecondaryUsage      *float64 `json:"secondary_usage_percent"`
	UsageUpdatedAt      int64    `json:"usage_updated_at"`
	UsageLastError      string   `json:"usage_last_error"`
	UsageErrorCode      string   `json:"usage_error_code"`
	UsageUpstreamStatus int      `json:"usage_upstream_status"`
	UsageCheckedAt      int64    `json:"usage_checked_at"`
	CreatedTime         int64    `json:"created_time"`
	UpdatedTime         int64    `json:"updated_time"`
	CredentialSet       bool     `json:"credential_set"`
	CredentialPreview   string   `json:"credential_preview"`
	BaseURL             string   `json:"base_url"`
	ModelMapping        string   `json:"model_mapping"`
}

type providerAccountSummaryView struct {
	providerAccountView
	PoolName        string `json:"pool_name"`
	PoolGroup       string `json:"pool_group"`
	PoolAdapterType int    `json:"pool_adapter_type"`
	PoolStatus      int    `json:"pool_status"`
	ChannelCount    int64  `json:"channel_count"`
}

type providerAccountBatchInput struct {
	AccountIds []int `json:"account_ids"`
	PoolId     int   `json:"pool_id"`
	Status     int   `json:"status"`
}

type providerAccountRoutingInput struct {
	Priority         int64 `json:"priority"`
	Weight           uint  `json:"weight"`
	ConcurrencyLimit int   `json:"concurrency_limit"`
	CooldownSeconds  int   `json:"cooldown_seconds"`
}

type providerAccountImportInput struct {
	PoolId   int                    `json:"pool_id"`
	Accounts []providerAccountInput `json:"accounts"`
}

type providerAccountUsageRefreshInput struct {
	AccountIds []int `json:"account_ids"`
}

func ListAccountPoolChannels(c *gin.Context) {
	var channels []struct {
		Id     int    `json:"id"`
		Name   string `json:"name"`
		Group  string `json:"group"`
		Status int    `json:"status"`
		Type   int    `json:"type"`
	}
	if err := model.DB.Model(&model.Channel{}).Select("id", "name", "group", "status", "type").Order("id DESC").Find(&channels).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": channels})
}

func ListAccountPools(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	pools, total, err := model.ListAccountPools(c.Query("keyword"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(pools)
	common.ApiSuccess(c, pageInfo)
}

func ListProviderAccounts(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	poolId, _ := strconv.Atoi(c.Query("pool_id"))
	status, _ := strconv.Atoi(c.Query("status"))
	accounts, total, err := model.ListProviderAccounts(c.Query("keyword"), poolId, status, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]providerAccountSummaryView, 0, len(accounts))
	for _, account := range accounts {
		views = append(views, providerAccountSummaryView{
			providerAccountView: providerAccountViewFromModel(account.ProviderAccount),
			PoolName:            account.PoolName, PoolGroup: account.PoolGroup,
			PoolAdapterType: account.PoolAdapterType, PoolStatus: account.PoolStatus,
			ChannelCount: account.ChannelCount,
		})
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(views)
	common.ApiSuccess(c, pageInfo)
}

func GetAccountPool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pool, accounts, channelIds, err := model.GetAccountPoolDetail(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]providerAccountView, 0, len(accounts))
	for _, account := range accounts {
		views = append(views, providerAccountViewFromModel(account))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"pool": pool, "accounts": views, "channel_ids": channelIds}})
}

func AssignProviderAccounts(c *gin.Context) {
	var input providerAccountBatchInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.AssignProviderAccountsToPool(input.AccountIds, input.PoolId); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitAccountPoolCache()
	recordManageAudit(c, "provider_account.assign", map[string]interface{}{"account_ids": input.AccountIds, "pool_id": input.PoolId})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UpdateProviderAccountStatuses(c *gin.Context) {
	var input providerAccountBatchInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateProviderAccountsStatus(input.AccountIds, input.Status); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitAccountPoolCache()
	recordManageAudit(c, "provider_account.status", map[string]interface{}{"account_ids": input.AccountIds, "status": input.Status})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteProviderAccounts(c *gin.Context) {
	var input providerAccountBatchInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	deleted, err := model.DeleteProviderAccounts(input.AccountIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitAccountPoolCache()
	recordManageAudit(c, "provider_account.delete_batch", map[string]interface{}{
		"account_ids": input.AccountIds,
		"count":       deleted,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"count": deleted}})
}

func UpdateProviderAccountRouting(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input providerAccountRoutingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateProviderAccountRouting(id, input.Priority, input.Weight, input.ConcurrencyLimit, input.CooldownSeconds); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitAccountPoolCache()
	recordManageAudit(c, "provider_account.routing", map[string]interface{}{
		"account_id": id, "priority": input.Priority, "weight": input.Weight,
		"concurrency_limit": input.ConcurrencyLimit, "cooldown_seconds": input.CooldownSeconds,
	})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func ImportProviderAccounts(c *gin.Context) {
	var input providerAccountImportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	pool, _, _, err := model.GetAccountPoolDetail(input.PoolId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	accounts := make([]model.ProviderAccount, 0, len(input.Accounts))
	for _, account := range input.Accounts {
		if account.AdapterType > 0 && pool.AdapterType > 0 && account.AdapterType != pool.AdapterType {
			common.ApiError(c, fmt.Errorf("account adapter type %d does not match target pool adapter type %d", account.AdapterType, pool.AdapterType))
			return
		}
		accounts = append(accounts, providerAccountModelFromInput(account))
	}
	result, err := model.ImportProviderAccountsWithResult(input.PoolId, accounts)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitAccountPoolCache()
	recordManageAudit(c, "provider_account.import", map[string]interface{}{
		"pool_id": input.PoolId, "account_count": result.Total,
		"created": result.Created, "updated": result.Updated, "skipped": result.Skipped,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"count": result.Created + result.Updated, "created": result.Created,
		"updated": result.Updated, "skipped": result.Skipped,
	}})
}

func GetProviderAccountUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	account, err := model.GetProviderAccountSummary(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.RefreshProviderAccountUsage(c.Request.Context(), account)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitAccountPoolCache()
	c.JSON(http.StatusOK, result)
}

func RefreshProviderAccountUsages(c *gin.Context) {
	var input providerAccountUsageRefreshInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	seen := make(map[int]struct{}, len(input.AccountIds))
	accountIds := make([]int, 0, len(input.AccountIds))
	for _, accountId := range input.AccountIds {
		if accountId <= 0 {
			continue
		}
		if _, exists := seen[accountId]; exists {
			continue
		}
		seen[accountId] = struct{}{}
		accountIds = append(accountIds, accountId)
	}
	if len(accountIds) == 0 {
		common.ApiError(c, fmt.Errorf("account_ids is required"))
		return
	}
	if len(accountIds) > 100 {
		common.ApiError(c, fmt.Errorf("no more than 100 provider accounts can be refreshed at once"))
		return
	}

	accounts, err := model.GetProviderAccountSummaries(accountIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	accountsById := make(map[int]*model.ProviderAccountSummary, len(accounts))
	for i := range accounts {
		accountsById[accounts[i].Id] = &accounts[i]
	}
	results := make([]service.ProviderAccountUsageRefreshResult, len(accountIds))
	group, groupCtx := errgroup.WithContext(c.Request.Context())
	group.SetLimit(10)
	for i, accountId := range accountIds {
		account, exists := accountsById[accountId]
		if !exists {
			results[i] = service.ProviderAccountUsageRefreshResult{
				AccountId: accountId, Message: "provider account not found", ErrorCode: "account_not_found",
			}
			continue
		}
		resultIndex := i
		providerAccount := account
		group.Go(func() error {
			result, refreshErr := service.RefreshProviderAccountUsage(groupCtx, providerAccount)
			if refreshErr != nil {
				result.Success = false
				result.ErrorCode = "refresh_internal_error"
				result.Message = refreshErr.Error()
			}
			results[resultIndex] = result
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitAccountPoolCache()

	succeeded := 0
	failed := 0
	unsupported := 0
	for _, result := range results {
		if result.Success {
			succeeded++
		} else if !result.Supported && result.ErrorCode == "" {
			unsupported++
		} else {
			failed++
		}
	}
	recordManageAudit(c, "provider_account.usage.refresh", map[string]interface{}{
		"account_ids": accountIds, "succeeded": succeeded, "failed": failed, "unsupported": unsupported,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total": len(results), "succeeded": succeeded, "failed": failed,
			"unsupported": unsupported, "results": results,
		},
	})
}

func CreateAccountPool(c *gin.Context) {
	var input accountPoolInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	pool, accounts := accountPoolModelsFromInput(input)
	if err := model.SaveAccountPool(pool, accounts, input.ChannelIds); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitAccountPoolCache()
	recordManageAudit(c, "account_pool.create", map[string]interface{}{"pool_id": pool.Id, "name": pool.Name, "account_count": len(accounts)})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"id": pool.Id}})
}

func UpdateAccountPool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input accountPoolInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	input.Id = id
	pool, accounts := accountPoolModelsFromInput(input)
	if err := model.SaveAccountPool(pool, accounts, input.ChannelIds); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitAccountPoolCache()
	recordManageAudit(c, "account_pool.update", map[string]interface{}{"pool_id": pool.Id, "name": pool.Name, "account_count": len(accounts)})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteAccountPool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteAccountPool(id); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitAccountPoolCache()
	recordManageAudit(c, "account_pool.delete", map[string]interface{}{"pool_id": id})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteProviderAccount(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteProviderAccount(id); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitAccountPoolCache()
	recordManageAudit(c, "provider_account.delete", map[string]interface{}{"account_id": id})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetProviderAccountCredential(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var account model.ProviderAccount
	if err := model.DB.First(&account, "id = ?", id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "provider_account.credential.read", map[string]interface{}{"account_id": id, "pool_id": account.PoolId})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"credential": account.Credential}})
}

func accountPoolModelsFromInput(input accountPoolInput) (*model.AccountPool, []model.ProviderAccount) {
	pool := &model.AccountPool{Id: input.Id, Name: input.Name, Provider: input.Provider, AdapterType: input.AdapterType, Group: input.Group, Status: input.Status, Priority: input.Priority, Weight: input.Weight, Remark: input.Remark}
	accounts := make([]model.ProviderAccount, 0, len(input.Accounts))
	for _, account := range input.Accounts {
		accounts = append(accounts, providerAccountModelFromInput(account))
	}
	return pool, accounts
}

func providerAccountModelFromInput(account providerAccountInput) model.ProviderAccount {
	return model.ProviderAccount{
		Id: account.Id, Name: account.Name, Type: account.Type, Credential: account.Credential,
		BaseURL: account.BaseURL, ModelMapping: account.ModelMapping,
		Status: account.Status, Priority: account.Priority, Weight: account.Weight,
		ConcurrencyLimit: account.ConcurrencyLimit, CooldownSeconds: account.CooldownSeconds,
		ExpiresAt: account.ExpiresAt, Metadata: account.Metadata,
	}
}

func providerAccountViewFromModel(account model.ProviderAccount) providerAccountView {
	preview := ""
	credential := strings.TrimSpace(account.Credential)
	if len(credential) > 4 {
		preview = "..." + credential[len(credential)-4:]
	} else if credential != "" {
		preview = "****"
	}
	planType, primaryUsage, secondaryUsage := providerAccountUsageSummary(account)
	return providerAccountView{
		Id: account.Id, PoolId: account.PoolId, Name: account.Name, Type: account.Type,
		Status: account.Status, Priority: account.Priority, Weight: account.Weight,
		ConcurrencyLimit: account.ConcurrencyLimit, CooldownSeconds: account.CooldownSeconds,
		ExpiresAt: account.ExpiresAt, LastUsedAt: account.LastUsedAt, LastError: account.LastError,
		Metadata: account.Metadata, CreatedTime: account.CreatedTime, UpdatedTime: account.UpdatedTime,
		PlanType: planType, PrimaryUsage: primaryUsage, SecondaryUsage: secondaryUsage,
		UsageUpdatedAt: account.UsageUpdatedAt, UsageLastError: account.UsageLastError,
		UsageErrorCode: account.UsageErrorCode, UsageUpstreamStatus: account.UsageUpstreamStatus,
		UsageCheckedAt: account.UsageCheckedAt,
		BaseURL:        account.BaseURL, ModelMapping: account.ModelMapping,
		CredentialSet: credential != "", CredentialPreview: preview,
	}
}

func providerAccountUsageSummary(account model.ProviderAccount) (string, *float64, *float64) {
	planType := ""
	if strings.TrimSpace(account.Metadata) != "" {
		var metadata map[string]interface{}
		if common.UnmarshalJsonStr(account.Metadata, &metadata) == nil {
			planType, _ = metadata["plan_type"].(string)
			if planType == "" {
				if extra, ok := metadata["extra"].(map[string]interface{}); ok {
					planType, _ = extra["plan_type"].(string)
				}
			}
		}
	}
	if strings.TrimSpace(account.UsageSnapshot) == "" {
		return planType, nil, nil
	}
	if strings.Contains(account.UsageSnapshot, `"requests"`) || strings.Contains(account.UsageSnapshot, `"tokens"`) || strings.Contains(account.UsageSnapshot, `"billing"`) {
		var snapshot map[string]interface{}
		if common.UnmarshalJsonStr(account.UsageSnapshot, &snapshot) == nil {
			if value, ok := snapshot["plan_type"].(string); ok && strings.TrimSpace(value) != "" {
				planType = strings.TrimSpace(value)
			}
			if billing, ok := snapshot["billing"].(map[string]interface{}); ok {
				for _, key := range []string{"plan", "plan_type", "planType"} {
					if value, exists := billing[key].(string); exists && strings.TrimSpace(value) != "" {
						planType = strings.TrimSpace(value)
						break
					}
				}
			}
			return planType, grokUsagePercent(snapshot, "requests"), grokUsagePercent(snapshot, "tokens")
		}
	}
	var snapshot struct {
		PlanType  string `json:"plan_type"`
		RateLimit struct {
			PlanType      string `json:"plan_type"`
			PrimaryWindow struct {
				UsedPercent *float64 `json:"used_percent"`
			} `json:"primary_window"`
			SecondaryWindow struct {
				UsedPercent *float64 `json:"used_percent"`
			} `json:"secondary_window"`
		} `json:"rate_limit"`
	}
	if common.UnmarshalJsonStr(account.UsageSnapshot, &snapshot) != nil {
		return planType, nil, nil
	}
	if strings.TrimSpace(snapshot.PlanType) != "" {
		planType = snapshot.PlanType
	} else if strings.TrimSpace(snapshot.RateLimit.PlanType) != "" {
		planType = snapshot.RateLimit.PlanType
	}
	return planType, snapshot.RateLimit.PrimaryWindow.UsedPercent, snapshot.RateLimit.SecondaryWindow.UsedPercent
}

func grokUsagePercent(snapshot map[string]interface{}, dimension string) *float64 {
	rateLimit, _ := snapshot["rate_limit"].(map[string]interface{})
	window, _ := rateLimit[dimension].(map[string]interface{})
	if window == nil {
		return nil
	}
	limit, limitOK := window["limit"].(float64)
	remaining, remainingOK := window["remaining"].(float64)
	if !limitOK || !remainingOK || limit <= 0 {
		return nil
	}
	percent := (limit - remaining) / limit * 100
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return &percent
}
