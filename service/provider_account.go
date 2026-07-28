package service

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	ginKeyProviderAccountLease        = "provider_account_lease"
	ginKeyFailedProviderAccountIDs    = "failed_provider_account_ids"
	defaultProviderAccount429Cooldown = 5
)

type ProviderAccountRoute struct {
	Credential   string
	BaseURL      string
	ModelMapping string
}

func ResolveChannelProviderAccount(c *gin.Context, channel *model.Channel) (ProviderAccountRoute, bool, *types.NewAPIError) {
	// A retry may move from a pooled channel to an unbound channel. Clear only
	// the selected route metadata here; the per-request failed-account set must
	// survive so a bad account cannot be selected again.
	common.SetContextKey(c, constant.ContextKeyAccountPoolId, 0)
	common.SetContextKey(c, constant.ContextKeyProviderAccountId, 0)
	common.SetContextKey(c, constant.ContextKeyProviderAccountName, "")
	common.SetContextKey(c, constant.ContextKeyProviderAccountCooldown, 0)

	group := common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	}
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	}
	failedAccountIDs, _ := c.Get(ginKeyFailedProviderAccountIDs)
	skippedAccountIDs, _ := failedAccountIDs.(map[int]struct{})
	lease, bound, err := model.AcquireProviderAccountWithOptions(channel.Id, channel.Type, group, model.ProviderAccountSelectionOptions{
		SkipAccountIDs:                   skippedAccountIDs,
		RequireGrokMediaGenerationAccess: requiresGrokMediaGenerationAccess(c, channel),
	})
	if err != nil {
		return ProviderAccountRoute{}, bound, types.NewErrorWithStatusCode(
			fmt.Errorf("account pool selection failed: %w", err),
			types.ErrorCodeGetChannelFailed,
			http.StatusServiceUnavailable,
		)
	}
	if !bound {
		return ProviderAccountRoute{}, false, nil
	}
	if lease == nil {
		return ProviderAccountRoute{}, true, types.NewErrorWithStatusCode(
			fmt.Errorf("account pool selection returned no account"),
			types.ErrorCodeGetChannelFailed,
			http.StatusServiceUnavailable,
		)
	}
	c.Set(ginKeyProviderAccountLease, lease)
	common.SetContextKey(c, constant.ContextKeyAccountPoolId, lease.PoolId)
	common.SetContextKey(c, constant.ContextKeyProviderAccountId, lease.AccountId)
	common.SetContextKey(c, constant.ContextKeyProviderAccountName, lease.AccountName)
	common.SetContextKey(c, constant.ContextKeyProviderAccountCooldown, lease.CooldownSeconds)
	return ProviderAccountRoute{Credential: lease.Credential, BaseURL: lease.BaseURL, ModelMapping: lease.ModelMapping}, true, nil
}

func requiresGrokMediaGenerationAccess(c *gin.Context, channel *model.Channel) bool {
	if c == nil || channel == nil || channel.Type != constant.ChannelTypeXai {
		return false
	}
	modelName := strings.ToLower(strings.TrimSpace(common.GetContextKeyString(c, constant.ContextKeyOriginalModel)))
	isMediaModel := strings.HasPrefix(modelName, "grok-imagine-image") ||
		strings.HasPrefix(modelName, "grok-2-image") ||
		strings.HasPrefix(modelName, "grok-imagine-video")
	if !isMediaModel {
		return false
	}
	relayMode, _ := c.Get("relay_mode")
	mode, _ := relayMode.(int)
	return mode == relayconstant.RelayModeImagesGenerations ||
		mode == relayconstant.RelayModeImagesEdits ||
		mode == relayconstant.RelayModeVideoSubmit
}

func ReleaseCurrentProviderAccountLease(c *gin.Context) {
	if c == nil {
		return
	}
	value, ok := c.Get(ginKeyProviderAccountLease)
	if !ok || value == nil {
		return
	}
	if lease, ok := value.(*model.ProviderAccountLease); ok {
		lease.Release()
	}
	c.Set(ginKeyProviderAccountLease, nil)
}

func MaybeCooldownSelectedProviderAccount(c *gin.Context, err *types.NewAPIError) {
	if c == nil || err == nil {
		return
	}
	accountId := common.GetContextKeyInt(c, constant.ContextKeyProviderAccountId)
	if accountId <= 0 {
		return
	}
	failedAccountIDs, _ := c.Get(ginKeyFailedProviderAccountIDs)
	skippedAccountIDs, _ := failedAccountIDs.(map[int]struct{})
	if skippedAccountIDs == nil {
		skippedAccountIDs = make(map[int]struct{})
		c.Set(ginKeyFailedProviderAccountIDs, skippedAccountIDs)
	}
	// Never retry the same provider account within one request. This covers
	// authentication failures and adapter/upstream errors alike; the channel
	// can still be retried with another account or a lower-priority channel.
	skippedAccountIDs[accountId] = struct{}{}
	// A provider-side 401 is retryable, but it must not permanently disable an
	// account. Exclude this account for the remainder of the request so the
	// retry can use another account or fall through to a lower-priority pool.
	if err.StatusCode == http.StatusUnauthorized {
		return
	}
	if !shouldCooldownChannelPool(err) {
		return
	}
	seconds := common.GetContextKeyInt(c, constant.ContextKeyProviderAccountCooldown)
	if seconds <= 0 && err.StatusCode == http.StatusTooManyRequests {
		seconds = defaultProviderAccount429Cooldown
	}
	if seconds <= 0 {
		return
	}
	model.CooldownProviderAccount(accountId, seconds, fmt.Sprintf("status=%d code=%s", err.StatusCode, err.GetErrorCode()))
}
