package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
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
	// In case some callers didn't normalize, keep a safety net.
	if HasCSAMViolationMarker(err) {
		return violationFeeReasonGrokCSAM, true
	}
	return "", false
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

// ChargeLocalViolationFee charges a request that YuAPI rejected before channel selection.
// The original sensitive-word error remains the client response even when charging fails.
func ChargeLocalViolationFee(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, apiErr *types.NewAPIError) bool {
	if ctx == nil || relayInfo == nil || apiErr == nil {
		return false
	}
	reason, charge := classifyViolationFee(apiErr)
	if !charge || reason != violationFeeReasonLocalSensitiveWord {
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

	if billingErr := PreConsumeBilling(ctx, feeQuota, relayInfo); billingErr != nil {
		logger.LogError(ctx, fmt.Sprintf("failed to charge local violation fee: %s", billingErr.Error()))
		return false
	}
	if err := SettleBilling(ctx, relayInfo, feeQuota); err != nil {
		logger.LogError(ctx, fmt.Sprintf("failed to settle local violation fee: %s", err.Error()))
		return false
	}

	model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, feeQuota)
	recordViolationFee(ctx, relayInfo, apiErr, reason, settings.ViolationDeductionAmount, groupRatio, feeQuota, 0)
	return true
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
