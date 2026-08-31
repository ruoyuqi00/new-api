package helper

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func modelPriceNotConfiguredError(modelName string, userId int) error {
	if model.IsAdmin(userId) {
		return fmt.Errorf(
			"模型 %s 的价格未配置。请前往「系统设置 → 运营设置」开启自用模式，或在「系统设置 → 分组与模型定价设置」中为该模型配置价格；"+
				"Model %s price not configured. Go to System Settings → Operation Settings to enable self-use mode, or configure the model price in System Settings → Group & Model Pricing.",
			modelName, modelName,
		)
	}
	return fmt.Errorf(
		"模型 %s 的价格尚未由管理员配置，暂时无法使用，请联系站点管理员开启该模型；"+
			"Model %s has not been priced by the administrator yet. Please contact the site administrator to enable this model.",
		modelName, modelName,
	)
}

// https://docs.claude.com/en/docs/build-with-claude/prompt-caching#1-hour-cache-duration
const claudeCacheCreation1hMultiplier = 6 / 3.75

// defaultTieredPreConsumeMaxTokens is the fallback completion-token estimate
// used for tiered expression pre-consume when the client omits max_tokens.
const defaultTieredPreConsumeMaxTokens = 8192

func ResolveEffectiveGroup(ctx *gin.Context, relayInfo *relaycommon.RelayInfo) string {
	if value, exists := ctx.Get("auto_group"); exists {
		if group, ok := value.(string); ok && group != "" {
			return group
		}
	}
	return relayInfo.UsingGroup
}

// HandleGroupRatio checks for "auto_group" in the context and updates the group ratio and relayInfo.UsingGroup if present
func HandleGroupRatio(ctx *gin.Context, relayInfo *relaycommon.RelayInfo) types.GroupRatioInfo {
	groupRatioInfo := types.GroupRatioInfo{
		GroupRatio:        1.0, // default ratio
		GroupSpecialRatio: -1,
	}

	effectiveGroup := ResolveEffectiveGroup(ctx, relayInfo)
	if effectiveGroup != relayInfo.UsingGroup {
		logger.LogDebug(ctx, "final group: %s", effectiveGroup)
		relayInfo.UsingGroup = effectiveGroup
	}

	// check user group special ratio
	userGroupRatio, ok := ratio_setting.GetUserGroupRatio(relayInfo.UserId, relayInfo.UserGroup, relayInfo.UsingGroup)
	if ok {
		// user group special ratio
		groupRatioInfo.GroupSpecialRatio = userGroupRatio
		groupRatioInfo.GroupRatio = userGroupRatio
		groupRatioInfo.HasSpecialRatio = true
	} else {
		// normal group ratio
		groupRatioInfo.GroupRatio = ratio_setting.GetGroupRatio(relayInfo.UsingGroup)
	}
	if snapshot := relayInfo.TieredBillingSnapshot; snapshot != nil {
		snapshot.GroupRatio = groupRatioInfo.GroupRatio
		snapshot.EstimatedQuotaAfterGroup = billingexpr.QuotaRound(
			snapshot.EstimatedQuotaBeforeGroup * groupRatioInfo.GroupRatio,
		)
	}

	return groupRatioInfo
}

func ModelPriceHelper(c *gin.Context, info *relaycommon.RelayInfo, promptTokens int, meta *types.TokenCountMeta) (types.PriceData, error) {
	modelPrice, usePrice := ratio_setting.GetModelPrice(info.OriginModelName, false)

	groupRatioInfo := HandleGroupRatio(c, info)

	billingMode := billing_setting.GetBillingMode(info.OriginModelName)
	if billing_setting.IsExpressionBillingMode(billingMode) {
		return modelPriceHelperExpression(c, info, promptTokens, meta, groupRatioInfo, billingMode)
	}

	imageResolutionQuote, imageResolutionPriceConfigured, err := resolveRequestImageResolutionPrice(info)
	if err != nil {
		return types.PriceData{}, err
	}
	if imageResolutionPriceConfigured {
		modelPrice = imageResolutionQuote.UnitPrice
		usePrice = true
	}

	var preConsumedQuota int
	var modelRatio float64
	var completionRatio float64
	var cacheRatio float64
	var imageRatio float64
	var cacheCreationRatio float64
	var cacheCreationRatio5m float64
	var cacheCreationRatio1h float64
	var audioRatio float64
	var audioCompletionRatio float64
	var freeModel bool
	if !usePrice {
		preConsumedTokens := common.Max(promptTokens, common.PreConsumedQuota)
		if meta.MaxTokens != 0 {
			preConsumedTokens += meta.MaxTokens
		}
		var success bool
		var matchName string
		modelRatio, success, matchName = ratio_setting.GetModelRatio(info.OriginModelName)
		if !success {
			acceptUnsetRatio := false
			if info.UserSetting.AcceptUnsetRatioModel {
				acceptUnsetRatio = true
			}
			if !acceptUnsetRatio {
				return types.PriceData{}, modelPriceNotConfiguredError(matchName, info.UserId)
			}
		}
		completionRatio = ratio_setting.GetCompletionRatio(info.OriginModelName)
		cacheRatio, _ = ratio_setting.GetCacheRatio(info.OriginModelName)
		cacheCreationRatio, _ = ratio_setting.GetCreateCacheRatio(info.OriginModelName)
		cacheCreationRatio5m = cacheCreationRatio
		// 固定1h和5min缓存写入价格的比例
		cacheCreationRatio1h = cacheCreationRatio * claudeCacheCreation1hMultiplier
		imageRatio, _ = ratio_setting.GetImageRatio(info.OriginModelName)
		audioRatio = ratio_setting.GetAudioRatio(info.OriginModelName)
		audioCompletionRatio = ratio_setting.GetAudioCompletionRatio(info.OriginModelName)
		ratio := modelRatio * groupRatioInfo.GroupRatio
		preConsumedQuota, err = common.QuotaFromFloatStrict(float64(preConsumedTokens) * ratio)
		if err != nil {
			return types.PriceData{}, fmt.Errorf("model %s pre-consume quota is out of range: %w", info.OriginModelName, err)
		}
	} else {
		if !imageResolutionPriceConfigured && meta.ImagePriceRatio != 0 {
			modelPrice = modelPrice * meta.ImagePriceRatio
		}
		preConsumedQuota, err = common.QuotaFromFloatStrict(modelPrice * common.QuotaPerUnit * groupRatioInfo.GroupRatio)
		if err != nil {
			return types.PriceData{}, fmt.Errorf("model %s pre-consume quota is out of range: %w", info.OriginModelName, err)
		}
	}

	// check if free model pre-consume is disabled
	if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume {
		// if model price or ratio is 0, do not pre-consume quota
		if groupRatioInfo.GroupRatio == 0 {
			preConsumedQuota = 0
			freeModel = true
		} else if usePrice {
			if modelPrice == 0 {
				preConsumedQuota = 0
				freeModel = true
			}
		} else {
			if modelRatio == 0 {
				preConsumedQuota = 0
				freeModel = true
			}
		}
	}

	priceData := types.PriceData{
		FreeModel:            freeModel,
		ModelPrice:           modelPrice,
		ModelRatio:           modelRatio,
		CompletionRatio:      completionRatio,
		GroupRatioInfo:       groupRatioInfo,
		UsePrice:             usePrice,
		CacheRatio:           cacheRatio,
		ImageRatio:           imageRatio,
		AudioRatio:           audioRatio,
		AudioCompletionRatio: audioCompletionRatio,
		CacheCreationRatio:   cacheCreationRatio,
		CacheCreation5mRatio: cacheCreationRatio5m,
		CacheCreation1hRatio: cacheCreationRatio1h,
		QuotaToPreConsume:    preConsumedQuota,
	}
	if imageResolutionPriceConfigured {
		imageCount := 1
		if count, ok := meta.BillingRatios["n"]; ok && count > 0 {
			imageCount = int(count)
		}
		priceData.ImageResolutionPricingModel = imageResolutionQuote.PricingModel
		priceData.ImageResolutionRequestedSize = imageResolutionQuote.NormalizedSize
		priceData.ImageResolutionTier = string(imageResolutionQuote.Tier)
		priceData.ImageResolutionUnitPrice = imageResolutionQuote.UnitPrice
		priceData.ImageResolutionImageCount = imageCount
	}
	if usePrice {
		quotaToPreConsume := modelPrice * common.QuotaPerUnit * groupRatioInfo.GroupRatio
		for name, ratio := range meta.BillingRatios {
			priceData.AddOtherRatio(name, ratio)
		}
		for _, ratio := range priceData.OtherRatios {
			quotaToPreConsume *= ratio
		}
		quota, err := common.QuotaFromFloatStrict(quotaToPreConsume)
		if err != nil {
			return types.PriceData{}, fmt.Errorf("model %s pre-consume quota is out of range: %w", info.OriginModelName, err)
		}
		priceData.QuotaToPreConsume = quota
	}

	if common.DebugEnabled {
		logger.LogDebug(c, "model_price_helper result: %s", priceData.ToSetting())
	}
	info.PriceData = priceData
	return priceData, nil
}

func resolveRequestImageResolutionPrice(info *relaycommon.RelayInfo) (operation_setting.ImageResolutionPriceQuote, bool, error) {
	if info == nil {
		return operation_setting.ImageResolutionPriceQuote{}, false, nil
	}
	var size string
	switch request := info.Request.(type) {
	case *dto.ImageRequest:
		size = request.Size
	case *dto.GeneralOpenAIRequest:
		size = request.Size
	default:
		return operation_setting.ImageResolutionPriceQuote{}, false, nil
	}
	return operation_setting.ResolveImageResolutionPrice(info.OriginModelName, size)
}

// ModelPriceHelperPerCall 按次/按量计费的 PriceHelper (MJ、Task)
func ModelPriceHelperPerCall(c *gin.Context, info *relaycommon.RelayInfo) (types.PriceData, error) {
	groupRatioInfo := HandleGroupRatio(c, info)

	modelPrice, success := ratio_setting.GetModelPrice(info.OriginModelName, true)
	usePrice := success
	var modelRatio float64

	if !success {
		defaultPrice, ok := ratio_setting.GetDefaultModelPriceMap()[info.OriginModelName]
		if ok {
			modelPrice = defaultPrice
			usePrice = true
		} else {
			var ratioSuccess bool
			var matchName string
			modelRatio, ratioSuccess, matchName = ratio_setting.GetModelRatio(info.OriginModelName)
			acceptUnsetRatio := false
			if info.UserSetting.AcceptUnsetRatioModel {
				acceptUnsetRatio = true
			}
			if !ratioSuccess && !acceptUnsetRatio {
				return types.PriceData{}, modelPriceNotConfiguredError(matchName, info.UserId)
			}
		}
	}

	var quota int
	freeModel := false

	if usePrice {
		var err error
		quota, err = common.QuotaFromFloatStrict(modelPrice * common.QuotaPerUnit * groupRatioInfo.GroupRatio)
		if err != nil {
			return types.PriceData{}, fmt.Errorf("model %s pre-consume quota is out of range: %w", info.OriginModelName, err)
		}
		if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume {
			if groupRatioInfo.GroupRatio == 0 || modelPrice == 0 {
				quota = 0
				freeModel = true
			}
		}
	} else {
		// 按量计费：以模型倍率的一半作为预扣额度
		var err error
		quota, err = common.QuotaFromFloatStrict(modelRatio / 2 * common.QuotaPerUnit * groupRatioInfo.GroupRatio)
		if err != nil {
			return types.PriceData{}, fmt.Errorf("model %s pre-consume quota is out of range: %w", info.OriginModelName, err)
		}
		modelPrice = -1
		if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume {
			if groupRatioInfo.GroupRatio == 0 || modelRatio == 0 {
				quota = 0
				freeModel = true
			}
		}
	}

	priceData := types.PriceData{
		FreeModel:      freeModel,
		ModelPrice:     modelPrice,
		ModelRatio:     modelRatio,
		UsePrice:       usePrice,
		Quota:          quota,
		GroupRatioInfo: groupRatioInfo,
	}
	return priceData, nil
}

func HasModelBillingConfig(modelName string) bool {
	if _, ok := ratio_setting.GetModelPrice(modelName, false); ok {
		return true
	}
	if _, ok, _ := ratio_setting.GetModelRatio(modelName); ok {
		return true
	}
	if !billing_setting.IsExpressionBillingMode(billing_setting.GetBillingMode(modelName)) {
		return false
	}
	expr, ok := billing_setting.GetBillingExpr(modelName)
	return ok && strings.TrimSpace(expr) != ""
}

func modelPriceHelperExpression(c *gin.Context, info *relaycommon.RelayInfo, promptTokens int, meta *types.TokenCountMeta, groupRatioInfo types.GroupRatioInfo, billingMode string) (types.PriceData, error) {
	exprStr, ok := billing_setting.GetBillingExpr(info.OriginModelName)
	if !ok {
		return types.PriceData{}, fmt.Errorf("model %s is configured as %s but has no billing expression", info.OriginModelName, billingMode)
	}
	if err := billing_setting.ValidateExpressionMode(billingMode, exprStr); err != nil {
		return types.PriceData{}, fmt.Errorf("model %s has invalid %s expression: %w", info.OriginModelName, billingMode, err)
	}

	estimatedCompletionTokens := meta.MaxTokens
	if billingMode == billing_setting.BillingModePerCallExpr {
		estimatedCompletionTokens = 0
	} else if estimatedCompletionTokens == 0 && groupRatioInfo.GroupRatio != 0 {
		estimatedCompletionTokens = defaultTieredPreConsumeMaxTokens
	}

	requestInput, err := ResolveIncomingBillingExprRequestInput(c, info)
	if err != nil {
		return types.PriceData{}, err
	}

	rawCost, trace, err := billingexpr.RunExprWithRequest(exprStr, billingexpr.TokenParams{
		P:   float64(promptTokens),
		C:   float64(estimatedCompletionTokens),
		Len: float64(promptTokens),
	}, requestInput)
	if err != nil {
		return types.PriceData{}, fmt.Errorf("model %s %s run failed: %w", info.OriginModelName, billingMode, err)
	}

	quotaBeforeGroup := billingexpr.ExpressionOutputToQuota(billingMode, rawCost, common.QuotaPerUnit)
	preConsumedQuota, err := billingexpr.QuotaRoundStrict(quotaBeforeGroup * groupRatioInfo.GroupRatio)
	if err != nil {
		return types.PriceData{}, fmt.Errorf("model %s pre-consume quota is out of range: %w", info.OriginModelName, err)
	}

	freeModel := false
	if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume {
		if groupRatioInfo.GroupRatio == 0 {
			preConsumedQuota = 0
			freeModel = true
		}
	}

	exprHash := billingexpr.ExprHashString(exprStr)
	snapshot := &billingexpr.BillingSnapshot{
		BillingMode:               billingMode,
		ModelName:                 info.OriginModelName,
		ExprString:                exprStr,
		ExprHash:                  exprHash,
		GroupRatio:                groupRatioInfo.GroupRatio,
		EstimatedPromptTokens:     promptTokens,
		EstimatedCompletionTokens: estimatedCompletionTokens,
		EstimatedQuotaBeforeGroup: quotaBeforeGroup,
		EstimatedQuotaAfterGroup:  preConsumedQuota,
		EstimatedTier:             trace.MatchedTier,
		QuotaPerUnit:              common.QuotaPerUnit,
		ExprVersion:               billingexpr.ExprVersion(exprStr),
	}
	info.TieredBillingSnapshot = snapshot
	info.BillingRequestInput = &requestInput

	priceData := types.PriceData{
		FreeModel:         freeModel,
		GroupRatioInfo:    groupRatioInfo,
		QuotaToPreConsume: preConsumedQuota,
	}

	logger.LogDebug(c, "model_price_helper_expression result: model=%s mode=%s preConsume=%d quotaBeforeGroup=%.2f groupRatio=%.2f tier=%s", info.OriginModelName, billingMode, preConsumedQuota, quotaBeforeGroup, groupRatioInfo.GroupRatio, trace.MatchedTier)

	info.PriceData = priceData
	return priceData, nil
}
