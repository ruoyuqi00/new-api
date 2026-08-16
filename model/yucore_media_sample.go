package model

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	YucoreMediaSampleCollectionID   = "video-model-examples"
	YucoreMediaSampleCollectionName = "视频模型示例"
	YucoreMediaSampleTaskPrefix     = "yu_sample_"
	YucoreMediaSampleMode           = "admin-sample-import"
)

func normalizeYucoreMediaSampleSHA256(checksum string) (string, error) {
	checksum = strings.ToLower(strings.TrimSpace(checksum))
	if len(checksum) != 64 {
		return "", errors.New("invalid YuCore media sample checksum")
	}
	if _, err := hex.DecodeString(checksum); err != nil {
		return "", errors.New("invalid YuCore media sample checksum")
	}
	return checksum, nil
}

func YucoreMediaSampleTaskID(userID int, checksum string) (string, error) {
	checksum, err := normalizeYucoreMediaSampleSHA256(checksum)
	if err != nil || userID <= 0 {
		return "", errors.New("invalid YuCore media sample identity")
	}
	prefix := fmt.Sprintf("%s%d_", YucoreMediaSampleTaskPrefix, userID)
	if len(prefix) >= 64 {
		return "", errors.New("invalid YuCore media sample identity")
	}
	return prefix + checksum[:64-len(prefix)], nil
}

func YucoreMediaSampleFileName(checksum string) string {
	checksum, err := normalizeYucoreMediaSampleSHA256(checksum)
	if err != nil {
		return ""
	}
	return "sample_" + checksum + ".mp4"
}

func IsYucoreMediaSampleTask(task *YucoreMediaTask) bool {
	if task == nil || task.UserId <= 0 || task.Mode != YucoreMediaSampleMode {
		return false
	}
	metadata := yucoreMediaMetadataMap(task.Metadata)
	if imported, ok := metadata["imported_sample"].(bool); !ok || !imported {
		return false
	}
	if collectionID, ok := metadata["collection_id"].(string); !ok || collectionID != YucoreMediaSampleCollectionID {
		return false
	}
	checksum, ok := metadata["sha256"].(string)
	if !ok {
		return false
	}
	expectedTaskID, err := YucoreMediaSampleTaskID(task.UserId, checksum)
	return err == nil && task.TaskId == expectedTaskID
}

func CreateYucoreMediaSampleTask(userID int, modelID string, checksum string, size int64) (*YucoreMediaTask, error) {
	modelID = strings.TrimSpace(modelID)
	checksum, err := normalizeYucoreMediaSampleSHA256(checksum)
	if err != nil || userID <= 0 || modelID == "" || size <= 0 {
		return nil, errors.New("invalid YuCore media sample task")
	}
	taskID, err := YucoreMediaSampleTaskID(userID, checksum)
	if err != nil {
		return nil, err
	}
	managedFileName := YucoreMediaSampleFileName(checksum)
	assets, err := marshalYucoreMediaAssets([]YucoreMediaAsset{{
		Id:              taskID + "_asset_0",
		Kind:            "video",
		Url:             fmt.Sprintf("/api/yucore/media/tasks/%s/assets/0", taskID),
		ManagedFileName: managedFileName,
		Label:           modelID + " example",
		MimeType:        "video/mp4",
	}})
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	task := &YucoreMediaTask{
		TaskId:      taskID,
		UserId:      userID,
		Kind:        "video",
		Mode:        YucoreMediaSampleMode,
		ModelId:     modelID,
		Prompt:      YucoreMediaSampleCollectionName,
		AspectRatio: "auto",
		Format:      "mp4",
		Count:       1,
		Status:      YucoreMediaTaskStatusCompleted,
		Progress:    100,
		Cost:        0,
		Assets:      YucoreMediaAssets(assets),
		Inputs:      "[]",
		Metadata: mergeYucoreMediaMetadata("", map[string]any{
			"imported_sample": true,
			"collection_id":   YucoreMediaSampleCollectionID,
			"collection_name": YucoreMediaSampleCollectionName,
			"sha256":          checksum,
			"size":            size,
		}),
		CreatedTime: now,
		UpdatedTime: now,
	}
	if err := DB.Create(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}
