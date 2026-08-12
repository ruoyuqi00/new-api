package service

import (
	"errors"
	"maps"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"gorm.io/gorm/clause"
)

var ErrTaskSubmissionMayHaveBeenSent = errors.New("task submission may have been sent upstream")

// PersistTaskSubmissionIntent records the frozen task and billing context
// before any upstream request may be written. A request may refresh its own
// pending intent while retrying an explicit rejection; another request may not
// reuse the same public task ID.
func PersistTaskSubmissionIntent(info *relaycommon.RelayInfo, platform constant.TaskPlatform) (bool, error) {
	if info == nil || info.TaskRelayInfo == nil || info.PublicTaskID == "" {
		return false, errors.New("missing public task submission identity")
	}
	task := taskSubmissionRecord(info, platform, model.TaskStatusUnknown, FrozenTaskSubmissionQuota(info), "", nil)
	if info.TaskRelayInfo.SubmissionIntentPersisted {
		var existing model.Task
		if err := model.DB.Where("submission_key = ?", info.PublicTaskID).First(&existing).Error; err != nil {
			return false, err
		}
		if existing.TaskID != task.TaskID || existing.UserId != task.UserId ||
			existing.Action != task.Action || existing.Quota != task.Quota ||
			!sameTaskSubmissionBillingContext(existing.PrivateData.BillingContext, task.PrivateData.BillingContext) {
			return false, errors.New("task submission intent conflicts with frozen context")
		}
		result := model.DB.Model(&model.Task{}).
			Where("submission_key = ? AND submission_billing_state = ? AND submission_stage IN ?", info.PublicTaskID, model.TaskSubmissionBillingPending, []model.TaskSubmissionStage{model.TaskSubmissionStagePreWrite, model.TaskSubmissionStageRejected}).
			Updates(map[string]any{
				"platform":         task.Platform,
				"group":            task.Group,
				"channel_id":       task.ChannelId,
				"quota":            task.Quota,
				"action":           task.Action,
				"submission_stage": model.TaskSubmissionStagePreWrite,
				"properties":       task.Properties,
				"private_data":     task.PrivateData,
				"data":             task.Data,
			})
		if result.Error != nil {
			return false, result.Error
		}
		if result.RowsAffected != 1 {
			var refreshed model.Task
			if err := model.DB.Select("submission_billing_state", "submission_stage").Where("submission_key = ?", info.PublicTaskID).First(&refreshed).Error; err != nil {
				return false, err
			}
			if refreshed.SubmissionBillingState == nil || *refreshed.SubmissionBillingState != model.TaskSubmissionBillingPending ||
				refreshed.SubmissionStage == nil || *refreshed.SubmissionStage != model.TaskSubmissionStagePreWrite {
				return false, errors.New("task submission intent is no longer pending")
			}
		}
		return false, nil
	}

	result := model.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "submission_key"}},
		DoNothing: true,
	}).Create(task)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected != 1 {
		return false, errors.New("task submission identity already exists")
	}
	info.TaskRelayInfo.SubmissionIntentPersisted = true
	return true, nil
}

// MarkTaskSubmissionWritePossible closes the crash gap immediately before the
// HTTP client may write upstream. Submitted means the request might have left
// this process, so an interrupted row must never be auto-refunded.
func MarkTaskSubmissionWritePossible(info *relaycommon.RelayInfo) error {
	if info == nil || info.TaskRelayInfo == nil || info.PublicTaskID == "" || !info.TaskRelayInfo.SubmissionIntentPersisted {
		return errors.New("missing owned task submission intent")
	}
	result := model.DB.Model(&model.Task{}).
		Where("submission_key = ? AND submission_billing_state = ? AND submission_stage = ?", info.PublicTaskID, model.TaskSubmissionBillingPending, model.TaskSubmissionStagePreWrite).
		Update("submission_stage", model.TaskSubmissionStageWritePossible)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("task submission intent is no longer pre-write")
	}
	return nil
}

// MarkTaskSubmissionRejected records a definitive upstream rejection before
// retry or refund. A process restart can safely refund this stage.
func MarkTaskSubmissionRejected(info *relaycommon.RelayInfo) error {
	if info == nil || info.TaskRelayInfo == nil || info.PublicTaskID == "" || !info.TaskRelayInfo.SubmissionIntentPersisted {
		return errors.New("missing owned task submission intent")
	}
	result := model.DB.Model(&model.Task{}).
		Where("submission_key = ? AND submission_billing_state = ? AND submission_stage = ?", info.PublicTaskID, model.TaskSubmissionBillingPending, model.TaskSubmissionStageWritePossible).
		Update("submission_stage", model.TaskSubmissionStageRejected)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("task submission intent is no longer write-possible")
	}
	return nil
}

func sameTaskSubmissionBillingContext(existing, candidate *model.TaskBillingContext) bool {
	if existing == nil || candidate == nil {
		return existing == nil && candidate == nil
	}
	return existing.ModelPrice == candidate.ModelPrice &&
		existing.GroupRatio == candidate.GroupRatio &&
		existing.ModelRatio == candidate.ModelRatio &&
		existing.OriginModelName == candidate.OriginModelName &&
		existing.PerCallBilling == candidate.PerCallBilling &&
		maps.Equal(existing.OtherRatios, candidate.OtherRatios)
}

func CompleteTaskSubmission(info *relaycommon.RelayInfo, platform constant.TaskPlatform, status model.TaskStatus, quota int, upstreamTaskID string, taskData []byte) error {
	if info == nil || info.TaskRelayInfo == nil || info.PublicTaskID == "" || !info.TaskRelayInfo.SubmissionIntentPersisted {
		return errors.New("missing owned task submission intent")
	}
	task := taskSubmissionRecord(info, platform, status, quota, upstreamTaskID, taskData)
	result := model.DB.Model(&model.Task{}).
		Where("submission_key = ? AND submission_billing_state = ? AND submission_stage IN ?", info.PublicTaskID, model.TaskSubmissionBillingPending, []model.TaskSubmissionStage{model.TaskSubmissionStagePreWrite, model.TaskSubmissionStageWritePossible}).
		Updates(map[string]any{
			"platform":         task.Platform,
			"group":            task.Group,
			"channel_id":       task.ChannelId,
			"quota":            task.Quota,
			"action":           task.Action,
			"status":           task.Status,
			"submission_stage": model.TaskSubmissionStageCompleted,
			"properties":       task.Properties,
			"private_data":     task.PrivateData,
			"data":             task.Data,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("task submission intent is no longer pending")
	}
	return nil
}

func DeleteTaskSubmissionIntent(info *relaycommon.RelayInfo) error {
	if info == nil || info.TaskRelayInfo == nil || info.PublicTaskID == "" || !info.TaskRelayInfo.SubmissionIntentPersisted {
		return nil
	}
	result := model.DB.Where("submission_key = ? AND submission_billing_state = ? AND submission_stage IN ?", info.PublicTaskID, model.TaskSubmissionBillingPending, []model.TaskSubmissionStage{model.TaskSubmissionStagePreWrite, model.TaskSubmissionStageRejected}).
		Delete(&model.Task{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		info.TaskRelayInfo.SubmissionIntentPersisted = false
		return nil
	}
	var task model.Task
	if err := model.DB.Select("submission_stage").Where("submission_key = ?", info.PublicTaskID).First(&task).Error; err != nil {
		return err
	}
	if task.SubmissionStage != nil && (*task.SubmissionStage == model.TaskSubmissionStageWritePossible || *task.SubmissionStage == model.TaskSubmissionStageCompleted) {
		return ErrTaskSubmissionMayHaveBeenSent
	}
	return errors.New("task submission intent is not refundable")
}

func taskSubmissionRecord(info *relaycommon.RelayInfo, platform constant.TaskPlatform, status model.TaskStatus, quota int, upstreamTaskID string, taskData []byte) *model.Task {
	submissionKey := info.PublicTaskID
	billingState := model.TaskSubmissionBillingPending
	task := model.InitTask(platform, info)
	task.SubmissionKey = &submissionKey
	task.SubmissionBillingState = &billingState
	submissionStage := model.TaskSubmissionStagePreWrite
	task.SubmissionStage = &submissionStage
	task.Status = status
	task.PrivateData.UpstreamTaskID = upstreamTaskID
	task.PrivateData.BillingSource = info.BillingSource
	task.PrivateData.SubscriptionId = info.SubscriptionId
	task.PrivateData.TokenId = info.TokenId
	task.PrivateData.NodeName = common.NodeName
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:      info.PriceData.ModelPrice,
		GroupRatio:      info.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio:      info.PriceData.ModelRatio,
		OtherRatios:     maps.Clone(info.PriceData.OtherRatios),
		OriginModelName: info.OriginModelName,
		PerCallBilling:  info.TaskPerCallBilling,
	}
	task.Quota = quota
	task.Data = taskData
	task.Action = info.Action
	return task
}
