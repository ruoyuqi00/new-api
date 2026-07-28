package service

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const ginKeyChannelPoolLease = "channel_pool_lease"

func TryAcquireChannelPoolLease(c *gin.Context, channel *model.Channel) (bool, error) {
	if c == nil || channel == nil {
		return false, fmt.Errorf("channel pool acquire needs a selected channel")
	}
	if existing, ok := c.Get(ginKeyChannelPoolLease); ok {
		if lease, ok := existing.(*model.ChannelPoolLease); ok && lease != nil {
			if lease.ChannelID() == channel.Id {
				logger.LogDebug(c, "channel pool lease reuse: channel #%d", channel.Id)
				return true, nil
			}
			logger.LogWarn(c, fmt.Sprintf("channel pool lease replaced: old_channel #%d new_channel #%d", lease.ChannelID(), channel.Id))
			lease.Release()
			c.Set(ginKeyChannelPoolLease, nil)
		}
	}
	group := channelPoolGroup(c)
	modelName := channelPoolModel(c)
	beforeAcquireStatus := model.ChannelPoolCandidateStatusFor(channel, group, modelName)
	lease, acquired, err := model.AcquireChannelPoolLease(channel)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("channel pool acquire failed open (channel #%d): %v", channel.Id, err))
		return true, nil
	}
	if !acquired {
		fullStatus := model.ChannelPoolCandidateStatusFor(channel, group, modelName)
		logger.LogWarn(c, channelPoolDecisionLogMessage("full", fullStatus, group, modelName))
		return false, nil
	}
	if lease != nil {
		c.Set(ginKeyChannelPoolLease, lease)
	}
	logger.LogDebug(c, channelPoolDecisionLogMessage("acquired", beforeAcquireStatus, group, modelName))
	return true, nil
}

func ReleaseCurrentChannelPoolLease(c *gin.Context) {
	if c == nil {
		return
	}
	ReleaseCurrentProviderAccountLease(c)
	existing, ok := c.Get(ginKeyChannelPoolLease)
	if !ok || existing == nil {
		return
	}
	lease, ok := existing.(*model.ChannelPoolLease)
	if ok && lease != nil {
		logger.LogDebug(c, "channel pool lease release: channel #%d", lease.ChannelID())
		lease.Release()
	}
	c.Set(ginKeyChannelPoolLease, nil)
}

func IsChannelPoolTemporarilyUnavailable(channel *model.Channel, group string, modelName string) bool {
	status := model.ChannelPoolCandidateStatusFor(channel, group, modelName)
	return !status.Available
}

func IsChannelPoolTemporarilyUnavailableWithContext(c *gin.Context, channel *model.Channel, group string, modelName string) bool {
	status := model.ChannelPoolCandidateStatusFor(channel, group, modelName)
	if !status.Available {
		logger.LogWarn(c, channelPoolDecisionLogMessage("skip", status, group, modelName))
	}
	return !status.Available
}

func NewChannelPoolFullError(channel *model.Channel) *types.NewAPIError {
	channelID := 0
	if channel != nil {
		channelID = channel.Id
	}
	return types.NewErrorWithStatusCode(
		fmt.Errorf("channel #%d concurrency limit reached", channelID),
		types.ErrorCodeGetChannelFailed,
		http.StatusTooManyRequests,
		types.ErrOptionWithSkipRetry(),
	)
}

func MaybeCooldownSelectedChannelPool(c *gin.Context, err *types.NewAPIError) {
	if c == nil || !shouldCooldownSelectedChannelPool(c, err) {
		return
	}
	settings, ok := common.GetContextKeyType[dto.ChannelOtherSettings](c, constant.ContextKeyChannelOtherSetting)
	if !ok || settings.ChannelPoolCooldownSeconds <= 0 {
		return
	}
	channelID := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	if channelID <= 0 {
		return
	}
	group := common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	}
	modelName := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	if group == "" || modelName == "" {
		return
	}
	reason := fmt.Sprintf("status=%d code=%s", err.StatusCode, err.GetErrorCode())
	model.CooldownChannelPool(channelID, group, modelName, settings.ChannelPoolCooldownSeconds, reason)
	logger.LogWarn(c, fmt.Sprintf("channel pool cooldown set: channel #%d group=%s model=%s seconds=%d reason=%s", channelID, group, modelName, settings.ChannelPoolCooldownSeconds, reason))
}

func shouldCooldownSelectedChannelPool(c *gin.Context, err *types.NewAPIError) bool {
	if !shouldCooldownChannelPool(err) {
		return false
	}
	if common.GetContextKeyInt(c, constant.ContextKeyProviderAccountId) > 0 {
		return false
	}
	return true
}

func shouldCooldownChannelPool(err *types.NewAPIError) bool {
	if err == nil || types.IsSkipRetryError(err) || types.IsChannelError(err) {
		return false
	}
	code := err.StatusCode
	if code == http.StatusTooManyRequests || code == 529 {
		return true
	}
	if code >= 500 && code <= 599 {
		return !operation_setting.IsAlwaysSkipRetryStatusCode(code)
	}
	if code == http.StatusConflict || code == http.StatusTooEarly {
		return true
	}
	return false
}

func channelPoolDecisionLogMessage(action string, status model.ChannelPoolCandidateStatus, group string, modelName string) string {
	return fmt.Sprintf("channel pool %s: channel #%d group=%s model=%s reason=%s limit=%d inflight=%d cooldown=%t hard_limit=%t",
		action,
		status.ChannelID,
		group,
		modelName,
		status.Reason,
		status.Limit,
		status.Inflight,
		status.CoolingDown,
		status.HasHardLimit,
	)
}

func channelPoolGroup(c *gin.Context) string {
	group := common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
	if group != "" {
		return group
	}
	return common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
}

func channelPoolModel(c *gin.Context) string {
	return common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
}
