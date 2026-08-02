package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

const (
	ViolationFeeCodePrefix     = "violation_fee."
	CSAMViolationMarker        = "Failed check: SAFETY_CHECK_TYPE"
	ContentViolatesUsageMarker = "Content violates usage guidelines"
)

type violationFeeReason string

const (
	violationFeeReasonLocalSensitiveWord violationFeeReason = "local_sensitive_word"
	violationFeeReasonGrokCSAM           violationFeeReason = "grok_csam"
	violationFeeReasonInputModeration    violationFeeReason = "input_moderation"
)

func IsViolationFeeCode(code types.ErrorCode) bool {
	return strings.HasPrefix(string(code), ViolationFeeCodePrefix)
}

func HasCSAMViolationMarker(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), CSAMViolationMarker) || strings.Contains(err.Error(), ContentViolatesUsageMarker) {
		return true
	}
	msg := err.ToOpenAIError().Message
	return strings.Contains(msg, CSAMViolationMarker) || strings.Contains(err.Error(), ContentViolatesUsageMarker)
}

func WrapAsViolationFeeGrokCSAM(err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		return nil
	}
	oai := err.ToOpenAIError()
	oai.Type = string(types.ErrorCodeViolationFeeGrokCSAM)
	oai.Code = string(types.ErrorCodeViolationFeeGrokCSAM)
	return types.WithOpenAIError(oai, err.StatusCode, types.ErrOptionWithSkipRetry())
}

// NormalizeViolationFeeError ensures:
// - if the CSAM marker is present, error.code is set to a stable violation-fee code and skip-retry is enabled.
// - if error.code already has the violation-fee prefix, skip-retry is enabled.
//
// It must be called before retry decision logic.
func NormalizeViolationFeeError(err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		return nil
	}

	if HasCSAMViolationMarker(err) {
		return WrapAsViolationFeeGrokCSAM(err)
	}

	if IsViolationFeeCode(err.GetErrorCode()) {
		oai := err.ToOpenAIError()
		return types.WithOpenAIError(oai, err.StatusCode, types.ErrOptionWithSkipRetry())
	}

	return err
}

func classifyViolationFee(err *types.NewAPIError) (violationFeeReason, bool) {
	if err == nil {
		return "", false
	}
	if err.GetErrorCode() == types.ErrorCodeSensitiveWordsDetected {
		return violationFeeReasonLocalSensitiveWord, true
	}
	if err.GetErrorCode() == types.ErrorCodeViolationFeeGrokCSAM {
		return violationFeeReasonGrokCSAM, true
	}
	if err.GetErrorCode() == types.ErrorCodeViolationFeeInputModeration {
		return violationFeeReasonInputModeration, true
	}
	// In case some callers didn't normalize, keep a safety net.
	if HasCSAMViolationMarker(err) {
		return violationFeeReasonGrokCSAM, true
	}
	return "", false
}

// ChargeInputModerationViolation charges the quota already estimated for the
// original request. The request remains blocked when billing cannot complete.
func ChargeInputModerationViolation(
	ctx *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	apiErr *types.NewAPIError,
	expectedQuota int,
	result InputModerationResult,
) bool {
	if ctx == nil || relayInfo == nil || apiErr == nil || expectedQuota < 0 {
		return false
	}
	reason, charge := classifyViolationFee(apiErr)
	if !charge || reason != violationFeeReasonInputModeration {
		return false
	}

	chargeSucceeded := expectedQuota == 0
	chargedQuota := 0
	if expectedQuota > 0 {
		if billingErr := PreConsumeBilling(ctx, expectedQuota, relayInfo); billingErr != nil {
			logger.LogError(ctx, fmt.Sprintf("failed to charge input moderation violation: %s", billingErr.Error()))
		} else if err := SettleBilling(ctx, relayInfo, expectedQuota); err != nil {
			logger.LogError(ctx, fmt.Sprintf("failed to settle input moderation violation: %s", err.Error()))
		} else {
			chargeSucceeded = true
			chargedQuota = expectedQuota
		}
	}

	model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, chargedQuota)
	recordInputModerationViolation(ctx, relayInfo, apiErr, result, expectedQuota, chargedQuota, chargeSucceeded)
	return chargeSucceeded
}

func recordInputModerationViolation(
	ctx *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	apiErr *types.NewAPIError,
	result InputModerationResult,
	requestedQuota int,
	chargedQuota int,
	chargeSucceeded bool,
) {
	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	categories := append([]string(nil), result.Categories...)
	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:      0,
		ModelName:      relayInfo.OriginModelName,
		TokenName:      ctx.GetString("token_name"),
		Quota:          chargedQuota,
		Content:        "Input moderation blocked",
		TokenId:        relayInfo.TokenId,
		UseTimeSeconds: int(useTimeSeconds),
		IsStream:       relayInfo.IsStream,
		Group:          relayInfo.UsingGroup,
		Other: map[string]any{
			"violation_fee":         true,
			"violation_fee_code":    string(apiErr.GetErrorCode()),
			"violation_fee_reason":  string(violationFeeReasonInputModeration),
			"moderation_model":      result.Model,
			"moderation_categories": categories,
			"requested_quota":       requestedQuota,
			"charged_quota":         chargedQuota,
			"charge_succeeded":      chargeSucceeded,
			"status_code":           apiErr.StatusCode,
		},
	})
}

func calcViolationFeeQuota(amount, groupRatio float64) int {
	if amount <= 0 {
		return 0
	}
	if groupRatio <= 0 {
		return 0
	}
	quota := decimal.NewFromFloat(amount).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Mul(decimal.NewFromFloat(groupRatio)).
		Round(0).
		IntPart()
	if quota <= 0 {
		return 0
	}
	return int(quota)
}

// CalculateLocalSensitiveInputQuota calculates only the estimated prompt cost.
// Per-call pricing cannot be split into input and output, so it is not charged.
func CalculateLocalSensitiveInputQuota(relayInfo *relaycommon.RelayInfo, promptTokens int) (int, error) {
	if relayInfo == nil || promptTokens <= 0 || relayInfo.PriceData.FreeModel {
		return 0, nil
	}

	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	if groupRatio <= 0 {
		return 0, nil
	}
	if relayInfo.TieredBillingSnapshot != nil {
		snapshot := relayInfo.TieredBillingSnapshot
		requestInput := billingexpr.RequestInput{}
		if relayInfo.BillingRequestInput != nil {
			requestInput = *relayInfo.BillingRequestInput
		}
		params := billingexpr.TokenParams{
			P:   float64(promptTokens),
			Len: float64(promptTokens),
		}
		rawCost, _, err := billingexpr.RunExprWithRequest(snapshot.ExprString, params, requestInput)
		if err != nil {
			return 0, fmt.Errorf("calculate sensitive input quota: %w", err)
		}
		// Remove any fixed request component while preserving the input-length tier.
		baselineCost, _, err := billingexpr.RunExprWithRequest(snapshot.ExprString, billingexpr.TokenParams{
			Len: float64(promptTokens),
		}, requestInput)
		if err != nil {
			return 0, fmt.Errorf("calculate sensitive input quota baseline: %w", err)
		}
		inputCost := rawCost - baselineCost
		if inputCost <= 0 {
			return 0, nil
		}
		quota, err := billingexpr.QuotaRoundStrict(inputCost / 1_000_000 * common.QuotaPerUnit * snapshot.GroupRatio)
		if err != nil {
			return 0, fmt.Errorf("calculate sensitive input quota: %w", err)
		}
		if quota == 0 {
			return 1, nil
		}
		return quota, nil
	}
	if relayInfo.PriceData.UsePrice || relayInfo.PriceData.ModelRatio <= 0 {
		return 0, nil
	}

	quota, err := common.QuotaFromFloatStrict(float64(promptTokens) * relayInfo.PriceData.ModelRatio * groupRatio)
	if err != nil {
		return 0, fmt.Errorf("calculate sensitive input quota: %w", err)
	}
	if quota == 0 {
		return 1, nil
	}
	return quota, nil
}

// ChargeLocalViolationFee charges a request that YuAPI rejected before channel selection.
// The original sensitive-word error remains the client response even when charging fails.
func ChargeLocalViolationFee(
	ctx *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	apiErr *types.NewAPIError,
	expectedQuota int,
	promptTokens int,
) bool {
	if ctx == nil || relayInfo == nil || apiErr == nil {
		return false
	}
	reason, charge := classifyViolationFee(apiErr)
	if !charge || reason != violationFeeReasonLocalSensitiveWord || expectedQuota < 0 {
		return false
	}

	chargeSucceeded := expectedQuota == 0
	chargedQuota := 0
	if expectedQuota > 0 {
		if billingErr := PreConsumeBilling(ctx, expectedQuota, relayInfo); billingErr != nil {
			logger.LogError(ctx, fmt.Sprintf("failed to charge local violation fee: %s", billingErr.Error()))
		} else if err := SettleBilling(ctx, relayInfo, expectedQuota); err != nil {
			logger.LogError(ctx, fmt.Sprintf("failed to settle local violation fee: %s", err.Error()))
		} else {
			chargeSucceeded = true
			chargedQuota = expectedQuota
		}
	}

	if chargedQuota > 0 {
		model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, chargedQuota)
	}
	recordLocalSensitiveViolation(ctx, relayInfo, apiErr, promptTokens, expectedQuota, chargedQuota, chargeSucceeded)
	return chargeSucceeded
}

func recordLocalSensitiveViolation(
	ctx *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	apiErr *types.NewAPIError,
	promptTokens int,
	requestedQuota int,
	chargedQuota int,
	chargeSucceeded bool,
) {
	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:      0,
		ModelName:      relayInfo.OriginModelName,
		TokenName:      ctx.GetString("token_name"),
		Quota:          chargedQuota,
		Content:        "Sensitive input blocked",
		TokenId:        relayInfo.TokenId,
		UseTimeSeconds: int(time.Now().Unix() - relayInfo.StartTime.Unix()),
		IsStream:       relayInfo.IsStream,
		Group:          relayInfo.UsingGroup,
		Other: map[string]any{
			"violation_fee":        true,
			"violation_fee_code":   string(apiErr.GetErrorCode()),
			"violation_fee_reason": string(violationFeeReasonLocalSensitiveWord),
			"billing_scope":        "input_tokens_only",
			"prompt_tokens":        promptTokens,
			"model_ratio":          relayInfo.PriceData.ModelRatio,
			"group_ratio":          relayInfo.PriceData.GroupRatioInfo.GroupRatio,
			"requested_quota":      requestedQuota,
			"charged_quota":        chargedQuota,
			"charge_succeeded":     chargeSucceeded,
			"status_code":          apiErr.StatusCode,
		},
	})
}

// ChargeViolationFeeIfNeeded charges an additional fee after the normal flow finishes (including refund).
// It uses Grok fee settings as the fee policy.
func ChargeViolationFeeIfNeeded(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, apiErr *types.NewAPIError) bool {
	if ctx == nil || relayInfo == nil || apiErr == nil {
		return false
	}
	//if relayInfo.IsPlayground {
	//	return false
	//}
	reason, charge := classifyViolationFee(apiErr)
	if !charge || reason != violationFeeReasonGrokCSAM {
		return false
	}

	settings := model_setting.GetGrokSettings()
	if settings == nil || !settings.ViolationDeductionEnabled {
		return false
	}

	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	feeQuota := calcViolationFeeQuota(settings.ViolationDeductionAmount, groupRatio)
	if feeQuota <= 0 {
		return false
	}

	if err := PostConsumeQuota(relayInfo, feeQuota, 0, true); err != nil {
		logger.LogError(ctx, fmt.Sprintf("failed to charge violation fee: %s", err.Error()))
		return false
	}

	model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, feeQuota)
	if relayInfo.ChannelId > 0 {
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, feeQuota)
	}
	recordViolationFee(ctx, relayInfo, apiErr, reason, settings.ViolationDeductionAmount, groupRatio, feeQuota, relayInfo.ChannelId)
	return true
}

func recordViolationFee(
	ctx *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	apiErr *types.NewAPIError,
	reason violationFeeReason,
	baseAmount float64,
	groupRatio float64,
	feeQuota int,
	channelID int,
) {
	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	tokenName := ctx.GetString("token_name")
	violationCode := apiErr.GetErrorCode()
	if reason == violationFeeReasonGrokCSAM {
		violationCode = types.ErrorCodeViolationFeeGrokCSAM
	}

	other := map[string]any{
		"violation_fee":        true,
		"violation_fee_code":   string(violationCode),
		"violation_fee_reason": string(reason),
		"fee_quota":            feeQuota,
		"base_amount":          baseAmount,
		"group_ratio":          groupRatio,
		"status_code":          apiErr.StatusCode,
	}
	if reason == violationFeeReasonGrokCSAM {
		oai := apiErr.ToOpenAIError()
		other["upstream_error_type"] = oai.Type
		other["upstream_error_code"] = fmt.Sprintf("%v", oai.Code)
		other["violation_fee_marker"] = CSAMViolationMarker
	}

	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:      channelID,
		ModelName:      relayInfo.OriginModelName,
		TokenName:      tokenName,
		Quota:          feeQuota,
		Content:        "Violation fee charged",
		TokenId:        relayInfo.TokenId,
		UseTimeSeconds: int(useTimeSeconds),
		IsStream:       relayInfo.IsStream,
		Group:          relayInfo.UsingGroup,
		Other:          other,
	})
}
