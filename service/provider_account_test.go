package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderAccount429FailsOverWithinRequestAndAppliesDefaultCooldown(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	require.NoError(t, model.DB.AutoMigrate(&model.AccountPool{}, &model.ProviderAccount{}, &model.ChannelAccountPoolBinding{}))

	pool := model.AccountPool{Name: "service-request-failover", Group: "pro", Status: model.AccountPoolStatusEnabled}
	require.NoError(t, model.DB.Create(&pool).Error)
	t.Cleanup(func() { _ = model.DeleteAccountPool(pool.Id) })
	primary := model.ProviderAccount{PoolId: pool.Id, Name: "primary", Credential: "primary-key", Status: model.ProviderAccountEnabled, Priority: 100}
	secondary := model.ProviderAccount{PoolId: pool.Id, Name: "secondary", Credential: "secondary-key", Status: model.ProviderAccountEnabled, Priority: 50}
	require.NoError(t, model.DB.Create(&primary).Error)
	require.NoError(t, model.DB.Create(&secondary).Error)
	const channelId = 99107
	require.NoError(t, model.DB.Create(&model.ChannelAccountPoolBinding{ChannelId: channelId, PoolId: pool.Id, Enabled: true}).Error)
	channel := &model.Channel{Id: channelId, Type: 1}

	context := providerAccountTestContext()
	route, bound, routeErr := ResolveChannelProviderAccount(context, channel)
	require.Nil(t, routeErr)
	require.True(t, bound)
	assert.Equal(t, "primary-key", route.Credential)
	assert.Equal(t, primary.Id, common.GetContextKeyInt(context, constant.ContextKeyProviderAccountId))

	rateLimitErr := types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	MaybeCooldownSelectedProviderAccount(context, rateLimitErr)
	ReleaseCurrentProviderAccountLease(context)
	route, bound, routeErr = ResolveChannelProviderAccount(context, channel)
	require.Nil(t, routeErr)
	require.True(t, bound)
	assert.Equal(t, "secondary-key", route.Credential)
	ReleaseCurrentProviderAccountLease(context)

	newContext := providerAccountTestContext()
	route, bound, routeErr = ResolveChannelProviderAccount(newContext, channel)
	require.Nil(t, routeErr)
	require.True(t, bound)
	assert.Equal(t, "secondary-key", route.Credential)
	ReleaseCurrentProviderAccountLease(newContext)
}

func TestProviderAccount401FailsOverWithoutDisablingAccount(t *testing.T) {
	context := providerAccountTestContext()
	common.SetContextKey(context, constant.ContextKeyProviderAccountId, 99108)

	authErr := types.NewErrorWithStatusCode(errors.New("token invalidated"), types.ErrorCodeBadResponseStatusCode, http.StatusUnauthorized)
	MaybeCooldownSelectedProviderAccount(context, authErr)
	failedAccountIDs, exists := context.Get(ginKeyFailedProviderAccountIDs)
	require.True(t, exists)
	assert.Contains(t, failedAccountIDs.(map[int]struct{}), 99108)
}

func TestProviderAccountTransportFailureExcludesAccountWithinRequest(t *testing.T) {
	context := providerAccountTestContext()
	common.SetContextKey(context, constant.ContextKeyProviderAccountId, 99112)
	transportErr := types.NewError(
		errors.New("connection reset by peer"),
		types.ErrorCodeDoRequestFailed,
	)

	MaybeCooldownSelectedProviderAccount(context, transportErr)

	failedAccountIDs, exists := context.Get(ginKeyFailedProviderAccountIDs)
	require.True(t, exists)
	assert.Contains(t, failedAccountIDs.(map[int]struct{}), 99112)
}

func TestProviderAccount5xxFailsOverAndReturns503WhenPoolIsExhausted(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})
	require.NoError(t, model.DB.AutoMigrate(&model.AccountPool{}, &model.ProviderAccount{}, &model.ChannelAccountPoolBinding{}))

	pool := model.AccountPool{Name: "service-5xx-failover", Group: "pro", Status: model.AccountPoolStatusEnabled}
	require.NoError(t, model.DB.Create(&pool).Error)
	t.Cleanup(func() { _ = model.DeleteAccountPool(pool.Id) })
	primary := model.ProviderAccount{Id: 9910901, PoolId: pool.Id, Name: "primary-5xx", Credential: "primary-5xx-key", Status: model.ProviderAccountEnabled, Priority: 100}
	secondary := model.ProviderAccount{Id: 9910902, PoolId: pool.Id, Name: "secondary-5xx", Credential: "secondary-5xx-key", Status: model.ProviderAccountEnabled, Priority: 50}
	require.NoError(t, model.DB.Create(&primary).Error)
	require.NoError(t, model.DB.Create(&secondary).Error)
	const channelId = 99109
	require.NoError(t, model.DB.Create(&model.ChannelAccountPoolBinding{ChannelId: channelId, PoolId: pool.Id, Enabled: true}).Error)
	channel := &model.Channel{Id: channelId, Type: constant.ChannelTypeOpenAI}
	context := providerAccountTestContext()

	route, bound, routeErr := ResolveChannelProviderAccount(context, channel)
	require.Nil(t, routeErr)
	require.True(t, bound)
	assert.Equal(t, primary.Id, common.GetContextKeyInt(context, constant.ContextKeyProviderAccountId))

	upstreamErr := types.NewErrorWithStatusCode(errors.New("upstream unavailable"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	MaybeCooldownSelectedProviderAccount(context, upstreamErr)
	ReleaseCurrentProviderAccountLease(context)
	route, bound, routeErr = ResolveChannelProviderAccount(context, channel)
	require.Nil(t, routeErr)
	require.True(t, bound)
	assert.Equal(t, "secondary-5xx-key", route.Credential)

	MaybeCooldownSelectedProviderAccount(context, upstreamErr)
	ReleaseCurrentProviderAccountLease(context)
	_, bound, routeErr = ResolveChannelProviderAccount(context, channel)
	require.True(t, bound)
	require.NotNil(t, routeErr)
	assert.Equal(t, http.StatusServiceUnavailable, routeErr.StatusCode)
}

func TestUnboundProviderRouteClearsPreviousAccountSelection(t *testing.T) {
	context := providerAccountTestContext()
	common.SetContextKey(context, constant.ContextKeyAccountPoolId, 10)
	common.SetContextKey(context, constant.ContextKeyProviderAccountId, 20)
	common.SetContextKey(context, constant.ContextKeyProviderAccountName, "stale")
	common.SetContextKey(context, constant.ContextKeyProviderAccountCooldown, 30)

	_, bound, routeErr := ResolveChannelProviderAccount(context, &model.Channel{Id: 99110, Type: constant.ChannelTypeOpenAI})
	require.Nil(t, routeErr)
	assert.False(t, bound)
	assert.Zero(t, common.GetContextKeyInt(context, constant.ContextKeyAccountPoolId))
	assert.Zero(t, common.GetContextKeyInt(context, constant.ContextKeyProviderAccountId))
	assert.Empty(t, common.GetContextKeyString(context, constant.ContextKeyProviderAccountName))
	assert.Zero(t, common.GetContextKeyInt(context, constant.ContextKeyProviderAccountCooldown))
}

func TestProviderAccountErrorsDoNotCooldownWholeChannelPool(t *testing.T) {
	context := providerAccountTestContext()
	serverErr := types.NewErrorWithStatusCode(errors.New("upstream unavailable"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)

	common.SetContextKey(context, constant.ContextKeyProviderAccountId, 99111)
	assert.False(t, shouldCooldownSelectedChannelPool(context, serverErr))

	rateLimitErr := types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	assert.False(t, shouldCooldownSelectedChannelPool(context, rateLimitErr))

	common.SetContextKey(context, constant.ContextKeyProviderAccountId, 0)
	assert.True(t, shouldCooldownSelectedChannelPool(context, serverErr))
	assert.True(t, shouldCooldownSelectedChannelPool(context, rateLimitErr))
}

func TestRequiresGrokMediaGenerationAccessOnlyForNewMediaRequests(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeXai}

	imageContext := providerAccountTestContext()
	common.SetContextKey(imageContext, constant.ContextKeyOriginalModel, "grok-imagine-image")
	imageContext.Set("relay_mode", relayconstant.RelayModeImagesGenerations)
	assert.True(t, requiresGrokMediaGenerationAccess(imageContext, channel))

	videoSubmitContext := providerAccountTestContext()
	common.SetContextKey(videoSubmitContext, constant.ContextKeyOriginalModel, "grok-imagine-video")
	videoSubmitContext.Set("relay_mode", relayconstant.RelayModeVideoSubmit)
	assert.True(t, requiresGrokMediaGenerationAccess(videoSubmitContext, channel))

	videoStatusContext := providerAccountTestContext()
	common.SetContextKey(videoStatusContext, constant.ContextKeyOriginalModel, "grok-imagine-video")
	videoStatusContext.Set("relay_mode", relayconstant.RelayModeVideoFetchByID)
	assert.False(t, requiresGrokMediaGenerationAccess(videoStatusContext, channel))

	textContext := providerAccountTestContext()
	common.SetContextKey(textContext, constant.ContextKeyOriginalModel, "grok-4.5")
	textContext.Set("relay_mode", relayconstant.RelayModeResponses)
	assert.False(t, requiresGrokMediaGenerationAccess(textContext, channel))
}

func providerAccountTestContext() *gin.Context {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(context, constant.ContextKeyUsingGroup, "pro")
	return context
}
