package controller

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

const providerAccountModelDiscoveryConcurrency = 5

type providerAccountModelDiscovery struct {
	AccountId   int      `json:"account_id"`
	AccountName string   `json:"account_name"`
	Success     bool     `json:"success"`
	Models      []string `json:"models"`
	Message     string   `json:"message,omitempty"`
}

type providerModelCoverage struct {
	Model        string `json:"model"`
	SupportCount int    `json:"support_count"`
}

type accountPoolModelDiscovery struct {
	PoolId            int                             `json:"pool_id"`
	PoolName          string                          `json:"pool_name"`
	ChannelId         int                             `json:"channel_id"`
	ChannelName       string                          `json:"channel_name"`
	TotalAccounts     int                             `json:"total_accounts"`
	SucceededAccounts int                             `json:"succeeded_accounts"`
	FailedAccounts    int                             `json:"failed_accounts"`
	Complete          bool                            `json:"complete"`
	CommonModels      []string                        `json:"common_models"`
	Coverage          []providerModelCoverage         `json:"coverage"`
	Accounts          []providerAccountModelDiscovery `json:"accounts"`
}

func GetAccountPoolModels(c *gin.Context) {
	poolId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	discovery, err := discoverAccountPoolModels(poolId, channelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "account_pool.models.discover", map[string]interface{}{
		"pool_id": poolId, "channel_id": discovery.ChannelId,
		"total_accounts": discovery.TotalAccounts, "succeeded_accounts": discovery.SucceededAccounts,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": discovery})
}

func discoverAccountPoolModels(poolId int, channelId int) (*accountPoolModelDiscovery, error) {
	pool, accounts, channelIds, err := model.GetAccountPoolDetail(poolId)
	if err != nil {
		return nil, err
	}
	template, err := resolveAccountPoolModelChannel(pool, channelIds, channelId)
	if err != nil {
		return nil, err
	}

	enabledAccounts := make([]model.ProviderAccount, 0, len(accounts))
	for _, account := range accounts {
		if account.Status == model.ProviderAccountEnabled {
			enabledAccounts = append(enabledAccounts, account)
		}
	}
	if len(enabledAccounts) == 0 {
		return nil, fmt.Errorf("账号池 #%d 没有启用账号", poolId)
	}

	result := &accountPoolModelDiscovery{
		PoolId: pool.Id, PoolName: pool.Name, ChannelId: template.Id, ChannelName: template.Name,
		TotalAccounts: len(enabledAccounts), Accounts: make([]providerAccountModelDiscovery, len(enabledAccounts)),
	}
	var healthChanged atomic.Bool
	var group errgroup.Group
	group.SetLimit(providerAccountModelDiscoveryConcurrency)
	for i := range enabledAccounts {
		i := i
		group.Go(func() error {
			account := enabledAccounts[i]
			accountResult := providerAccountModelDiscovery{AccountId: account.Id, AccountName: account.Name}
			accountChannel, err := providerAccountModelChannel(template, pool, account)
			if err != nil {
				accountResult.Message = err.Error()
				result.Accounts[i] = accountResult
				return nil
			}
			models, err := fetchCredentialUpstreamModelIDs(accountChannel)
			if err != nil {
				accountResult.Message = err.Error()
				var statusErr *upstreamHTTPStatusError
				if accountChannel.Type == constant.ChannelTypeCodex && account.Type == "oauth_json" &&
					errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusUnauthorized {
					checkedAt := common.GetTimestamp()
					if updateErr := model.UpdateProviderAccountUsageHealth(
						account.Id, "", checkedAt, http.StatusUnauthorized, "invalid_credential", err.Error(),
					); updateErr != nil {
						accountResult.Message = fmt.Sprintf("%s; persist account health: %v", accountResult.Message, updateErr)
					} else {
						healthChanged.Store(true)
					}
				}
				result.Accounts[i] = accountResult
				return nil
			}
			accountResult.Success = true
			accountResult.Models = providerAccountRoutableModels(models, account.ModelMapping)
			result.Accounts[i] = accountResult
			return nil
		})
	}
	_ = group.Wait()
	if healthChanged.Load() {
		model.InitAccountPoolCache()
	}

	coverage := make(map[string]int)
	for _, accountResult := range result.Accounts {
		if !accountResult.Success {
			result.FailedAccounts++
			continue
		}
		result.SucceededAccounts++
		for _, modelName := range accountResult.Models {
			coverage[modelName]++
		}
	}
	result.Complete = result.SucceededAccounts == result.TotalAccounts
	result.Coverage = make([]providerModelCoverage, 0, len(coverage))
	for modelName, supportCount := range coverage {
		result.Coverage = append(result.Coverage, providerModelCoverage{Model: modelName, SupportCount: supportCount})
		if supportCount == result.SucceededAccounts {
			result.CommonModels = append(result.CommonModels, modelName)
		}
	}
	slices.SortFunc(result.Coverage, func(a, b providerModelCoverage) int {
		if a.SupportCount != b.SupportCount {
			return b.SupportCount - a.SupportCount
		}
		return strings.Compare(a.Model, b.Model)
	})
	slices.Sort(result.CommonModels)
	return result, nil
}

func resolveAccountPoolModelChannel(pool *model.AccountPool, channelIds []int, requestedChannelId int) (*model.Channel, error) {
	if requestedChannelId > 0 {
		isExplicitlyBound := slices.Contains(channelIds, requestedChannelId)
		isUnboundAdapterPool := len(channelIds) == 0 && pool.AdapterType > 0
		if !isExplicitlyBound && !isUnboundAdapterPool {
			return nil, fmt.Errorf("渠道 #%d 未绑定账号池 #%d", requestedChannelId, pool.Id)
		}
		channel, err := model.GetChannelById(requestedChannelId, true)
		if err != nil {
			return nil, err
		}
		if pool.AdapterType > 0 && channel.Type != pool.AdapterType {
			return nil, fmt.Errorf("渠道 #%d 类型与账号池适配器不一致", requestedChannelId)
		}
		return channel, nil
	}
	for _, id := range channelIds {
		channel, err := model.GetChannelById(id, true)
		if err != nil {
			continue
		}
		if pool.AdapterType <= 0 || channel.Type == pool.AdapterType {
			return channel, nil
		}
	}
	if pool.AdapterType <= 0 {
		return nil, fmt.Errorf("账号池 #%d 没有可用于模型探测的适配器或绑定渠道", pool.Id)
	}
	return &model.Channel{Type: pool.AdapterType, Name: pool.Name}, nil
}

func providerAccountModelChannel(template *model.Channel, pool *model.AccountPool, account model.ProviderAccount) (*model.Channel, error) {
	channel := *template
	channel.Keys = nil
	channel.ChannelInfo = model.ChannelInfo{}
	if pool.AdapterType > 0 {
		channel.Type = pool.AdapterType
	}

	credential := strings.TrimSpace(account.Credential)
	if account.Type == "oauth_json" && channel.Type != constant.ChannelTypeCodex && channel.Type != constant.ChannelTypeXai {
		var oauthCredential map[string]interface{}
		if err := common.UnmarshalJsonStr(credential, &oauthCredential); err != nil {
			return nil, fmt.Errorf("账号 %s 的 OAuth 凭据不是有效 JSON", account.Name)
		}
		accessToken, _ := oauthCredential["access_token"].(string)
		credential = strings.TrimSpace(accessToken)
	}
	if credential == "" {
		return nil, fmt.Errorf("账号 %s 缺少可用于模型探测的凭据", account.Name)
	}
	channel.Key = credential
	if strings.TrimSpace(account.BaseURL) != "" {
		baseURL := strings.TrimRight(strings.TrimSpace(account.BaseURL), "/")
		channel.BaseURL = &baseURL
	}
	if account.Type == "oauth_json" && channel.Type == constant.ChannelTypeXai &&
		(strings.TrimSpace(account.BaseURL) == "" || strings.EqualFold(strings.TrimRight(account.BaseURL, "/"), "https://api.x.ai") || strings.EqualFold(strings.TrimRight(account.BaseURL, "/"), "https://api.x.ai/v1")) {
		baseURL := constant.GrokOAuthBaseURL
		channel.BaseURL = &baseURL
	}
	if strings.TrimSpace(account.ModelMapping) != "" {
		modelMapping := strings.TrimSpace(account.ModelMapping)
		channel.ModelMapping = &modelMapping
	}
	return &channel, nil
}

func providerAccountRoutableModels(upstreamModels []string, modelMapping string) []string {
	models := normalizeModelNames(upstreamModels)
	if strings.TrimSpace(modelMapping) == "" {
		return models
	}
	var mapping map[string]string
	if err := common.UnmarshalJsonStr(modelMapping, &mapping); err != nil {
		return models
	}
	upstreamSet := make(map[string]struct{}, len(models))
	for _, modelName := range models {
		upstreamSet[modelName] = struct{}{}
	}
	for publicModel, upstreamModel := range mapping {
		publicModel = strings.TrimSpace(publicModel)
		upstreamModel = strings.TrimSpace(upstreamModel)
		if publicModel == "" || upstreamModel == "" {
			continue
		}
		if _, ok := upstreamSet[upstreamModel]; ok {
			models = append(models, publicModel)
		}
	}
	return normalizeModelNames(models)
}
