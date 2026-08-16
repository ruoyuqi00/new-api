package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var yucoreMediaSampleImportLocks sync.Map

var errYucoreMediaSampleChecksumMismatch = errors.New("sample checksum mismatch")

func denyYucoreMediaSampleAccess(c *gin.Context, task *model.YucoreMediaTask) bool {
	if task == nil || !strings.HasPrefix(task.TaskId, model.YucoreMediaSampleTaskPrefix) {
		return false
	}
	if c.GetInt("role") < common.RoleAdminUser || !model.IsYucoreMediaSampleTask(task) {
		writeYucoreMediaTaskNotFound(c)
		return true
	}
	return false
}

func rejectYucoreMediaSampleMutation(c *gin.Context, task *model.YucoreMediaTask) bool {
	if denyYucoreMediaSampleAccess(c, task) {
		return true
	}
	if model.IsYucoreMediaSampleTask(task) {
		common.ApiErrorMsg(c, "managed sample tasks must be deleted through the sample rollback endpoint")
		return true
	}
	return false
}

func writeYucoreMediaTaskNotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "media task not found"})
}

func ImportYucoreMediaSample(c *gin.Context) {
	if c.GetInt("role") < common.RoleAdminUser {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "administrator access is required"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, yucoreMediaUploadRequestMaxBytes)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		common.ApiErrorMsg(c, "invalid sample upload")
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	form := c.Request.MultipartForm
	if form == nil || len(form.File) != 1 || len(form.File["file"]) != 1 {
		common.ApiErrorMsg(c, "exactly one sample video is required")
		return
	}
	for key, values := range form.File {
		if key != "file" || len(values) != 1 {
			common.ApiErrorMsg(c, "sample upload contains an unsupported file field")
			return
		}
	}
	allowedFields := map[string]struct{}{"model_id": {}, "sha256": {}, "collection_id": {}}
	for key, values := range form.Value {
		if _, ok := allowedFields[key]; !ok || len(values) != 1 {
			common.ApiErrorMsg(c, "sample upload contains an unsupported form field")
			return
		}
	}
	for key := range allowedFields {
		if len(form.Value[key]) != 1 {
			common.ApiErrorMsg(c, "sample upload is missing a required form field")
			return
		}
	}

	ownerID := c.GetInt("id")
	if ownerID <= 0 {
		common.ApiErrorMsg(c, "invalid sample owner")
		return
	}
	if strings.TrimSpace(form.Value["collection_id"][0]) != model.YucoreMediaSampleCollectionID {
		common.ApiErrorMsg(c, "invalid sample collection")
		return
	}
	modelID := strings.TrimSpace(form.Value["model_id"][0])
	_, capabilities := model.GetYucoreMediaCatalogSettings()
	capability, configured := capabilities[modelID]
	if !configured {
		for configuredModel, candidate := range capabilities {
			if strings.EqualFold(strings.TrimSpace(configuredModel), modelID) {
				modelID = strings.TrimSpace(configuredModel)
				capability = candidate
				configured = true
				break
			}
		}
	}
	if !configured || !strings.EqualFold(strings.TrimSpace(capability.Kind), "video") ||
		!strings.EqualFold(strings.TrimSpace(capability.Availability), model.YucoreMediaAvailabilityEnabled) {
		common.ApiErrorMsg(c, "sample model must be an enabled configured video model")
		return
	}

	checksum := strings.ToLower(strings.TrimSpace(form.Value["sha256"][0]))
	if len(checksum) != sha256.Size*2 {
		common.ApiErrorMsg(c, "invalid sample checksum")
		return
	}
	if _, err := hex.DecodeString(checksum); err != nil {
		common.ApiErrorMsg(c, "invalid sample checksum")
		return
	}
	taskID, err := model.YucoreMediaSampleTaskID(ownerID, checksum)
	if err != nil {
		common.ApiErrorMsg(c, "invalid sample identity")
		return
	}
	managedFileName := model.YucoreMediaSampleFileName(checksum)
	finalPath, err := yucoreMediaSafeUploadPath(ownerID, managedFileName)
	if err != nil {
		common.ApiErrorMsg(c, "invalid managed sample path")
		return
	}

	fileHeader := form.File["file"][0]
	if fileHeader.Size <= 0 {
		common.ApiErrorMsg(c, "sample upload is empty")
		return
	}
	if fileHeader.Size > maxYucoreMediaVideoUploadBytes {
		common.ApiErrorMsg(c, "sample video exceeds the allowed size")
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		common.ApiErrorMsg(c, "sample video could not be opened")
		return
	}
	defer file.Close()
	prefix := make([]byte, yucoreMediaUploadSniffBytes)
	prefixLength, err := io.ReadFull(file, prefix)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		common.ApiErrorMsg(c, "sample video could not be read")
		return
	}
	prefix = prefix[:prefixLength]
	policy, err := yucoreMediaUploadPolicyFor(prefix, fileHeader.Header.Get("Content-Type"))
	if err != nil || policy.Kind != "video" || policy.MIMEType != "video/mp4" || policy.Extension != ".mp4" {
		common.ApiErrorMsg(c, "sample upload must be a valid MP4 video")
		return
	}

	lockValue, _ := yucoreMediaSampleImportLocks.LoadOrStore(taskID, &sync.Mutex{})
	importLock := lockValue.(*sync.Mutex)
	importLock.Lock()
	defer importLock.Unlock()

	var existing model.YucoreMediaTask
	existingErr := model.DB.Where("task_id = ? AND user_id = ?", taskID, ownerID).First(&existing).Error
	if existingErr != nil && !errors.Is(existingErr, gorm.ErrRecordNotFound) {
		common.ApiErrorMsg(c, "sample task lookup failed")
		return
	}

	hasher := sha256.New()
	reader := io.TeeReader(io.MultiReader(bytes.NewReader(prefix), file), hasher)
	if existingErr == nil {
		written, copyErr := io.Copy(io.Discard, io.LimitReader(reader, maxYucoreMediaVideoUploadBytes+1))
		if copyErr != nil || written > maxYucoreMediaVideoUploadBytes {
			common.ApiErrorMsg(c, "sample video exceeds the allowed size")
			return
		}
		if hex.EncodeToString(hasher.Sum(nil)) != checksum {
			common.ApiErrorMsg(c, "sample checksum does not match the uploaded video")
			return
		}
		if err := validateExistingYucoreMediaSample(&existing, ownerID, modelID, checksum, finalPath, written); err != nil {
			common.ApiErrorMsg(c, "sample import conflict")
			return
		}
		writeYucoreMediaSampleImportSuccess(c, &existing, checksum, written, false)
		return
	}

	written, err := storeYucoreMediaUploadValidated(reader, finalPath, maxYucoreMediaVideoUploadBytes, func(_ int64) error {
		if hex.EncodeToString(hasher.Sum(nil)) != checksum {
			return errYucoreMediaSampleChecksumMismatch
		}
		return nil
	})
	if errors.Is(err, errYucoreMediaUploadTooLarge) {
		common.ApiErrorMsg(c, "sample video exceeds the allowed size")
		return
	}
	if err != nil {
		if errors.Is(err, errYucoreMediaSampleChecksumMismatch) {
			common.ApiErrorMsg(c, "sample checksum does not match the uploaded video")
			return
		}
		var raced model.YucoreMediaTask
		if lookupErr := model.DB.Where("task_id = ? AND user_id = ?", taskID, ownerID).First(&raced).Error; lookupErr == nil {
			if validateExistingYucoreMediaSample(&raced, ownerID, modelID, checksum, finalPath, written) == nil {
				writeYucoreMediaSampleImportSuccess(c, &raced, checksum, written, false)
				return
			}
		}
		common.ApiErrorMsg(c, "sample video could not be stored")
		return
	}
	task, createErr := model.CreateYucoreMediaSampleTask(ownerID, modelID, checksum, written)
	if createErr == nil {
		writeYucoreMediaSampleImportSuccess(c, task, checksum, written, true)
		return
	}

	var conflicted model.YucoreMediaTask
	conflictErr := model.DB.Where("task_id = ? AND user_id = ?", taskID, ownerID).First(&conflicted).Error
	if conflictErr == nil && validateExistingYucoreMediaSample(&conflicted, ownerID, modelID, checksum, finalPath, written) == nil {
		writeYucoreMediaSampleImportSuccess(c, &conflicted, checksum, written, false)
		return
	}
	if conflictErr == nil && validateExistingYucoreMediaSample(&conflicted, ownerID, "", checksum, finalPath, written) == nil {
		common.ApiErrorMsg(c, "sample import conflict")
		return
	}
	_ = os.Remove(finalPath)
	common.ApiErrorMsg(c, "sample task could not be created")
}

func DeleteYucoreMediaSample(c *gin.Context) {
	if c.GetInt("role") < common.RoleAdminUser {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "administrator access is required"})
		return
	}
	ownerID := c.GetInt("id")
	taskID := strings.TrimSpace(c.Param("task_id"))
	if ownerID <= 0 || taskID == "" {
		common.ApiErrorMsg(c, "invalid sample identity")
		return
	}
	lockValue, _ := yucoreMediaSampleImportLocks.LoadOrStore(taskID, &sync.Mutex{})
	rollbackLock := lockValue.(*sync.Mutex)
	rollbackLock.Lock()
	defer rollbackLock.Unlock()

	var task model.YucoreMediaTask
	if err := model.DB.Where("task_id = ? AND user_id = ?", taskID, ownerID).First(&task).Error; err != nil {
		common.ApiErrorMsg(c, "sample task not found")
		return
	}
	checksum, finalPath, err := yucoreMediaManagedSamplePath(&task, ownerID)
	if err != nil {
		common.ApiErrorMsg(c, "sample task is not managed")
		return
	}
	if checksum == "" {
		common.ApiErrorMsg(c, "sample task is not managed")
		return
	}
	info, err := os.Lstat(finalPath)
	if err != nil || !info.Mode().IsRegular() {
		common.ApiErrorMsg(c, "managed sample file is unavailable")
		return
	}
	quarantine, err := os.CreateTemp(filepath.Dir(finalPath), ".yucore-sample-rollback-*.mp4")
	if err != nil {
		common.ApiErrorMsg(c, "managed sample rollback could not start")
		return
	}
	quarantinePath := quarantine.Name()
	if closeErr := quarantine.Close(); closeErr != nil {
		_ = os.Remove(quarantinePath)
		common.ApiErrorMsg(c, "managed sample rollback could not start")
		return
	}
	if err := os.Remove(quarantinePath); err != nil {
		common.ApiErrorMsg(c, "managed sample rollback could not start")
		return
	}
	if err := os.Rename(finalPath, quarantinePath); err != nil {
		common.ApiErrorMsg(c, "managed sample rollback could not start")
		return
	}

	deleteResult := model.DB.Where("id = ? AND user_id = ?", task.Id, ownerID).Delete(&model.YucoreMediaTask{})
	if deleteResult.Error != nil || deleteResult.RowsAffected != 1 {
		_ = os.Rename(quarantinePath, finalPath)
		common.ApiErrorMsg(c, "sample task rollback failed")
		return
	}
	if err := os.Remove(quarantinePath); err != nil {
		restoreDBErr := model.DB.Unscoped().Model(&model.YucoreMediaTask{}).
			Where("id = ? AND user_id = ?", task.Id, ownerID).
			Update("deleted_at", nil).Error
		restoreFileErr := os.Rename(quarantinePath, finalPath)
		if restoreDBErr != nil || restoreFileErr != nil {
			common.ApiErrorMsg(c, "sample rollback recovery failed")
			return
		}
		common.ApiErrorMsg(c, "sample rollback could not remove the managed file")
		return
	}
	common.ApiSuccess(c, nil)
}

func validateExistingYucoreMediaSample(task *model.YucoreMediaTask, ownerID int, modelID string, checksum string, expectedPath string, expectedSize int64) error {
	storedChecksum, storedPath, err := yucoreMediaManagedSamplePath(task, ownerID)
	if err != nil || storedChecksum != checksum || storedPath != expectedPath || (modelID != "" && task.ModelId != modelID) {
		return errors.New("sample identity conflict")
	}
	linkInfo, err := os.Lstat(storedPath)
	if err != nil || !linkInfo.Mode().IsRegular() {
		return errors.New("managed sample file is invalid")
	}
	file, err := os.Open(storedPath)
	if err != nil {
		return errors.New("managed sample file is missing")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != expectedSize {
		return errors.New("managed sample file is invalid")
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil || hex.EncodeToString(hasher.Sum(nil)) != checksum {
		return errors.New("managed sample checksum mismatch")
	}
	return nil
}

func yucoreMediaManagedSamplePath(task *model.YucoreMediaTask, ownerID int) (string, string, error) {
	if task == nil || task.UserId != ownerID || !model.IsYucoreMediaSampleTask(task) {
		return "", "", errors.New("invalid managed sample task")
	}
	metadata := map[string]any{}
	if err := common.Unmarshal([]byte(task.Metadata), &metadata); err != nil {
		return "", "", errors.New("invalid managed sample metadata")
	}
	checksum, ok := metadata["sha256"].(string)
	if !ok {
		return "", "", errors.New("invalid managed sample checksum")
	}
	checksum = strings.ToLower(strings.TrimSpace(checksum))
	if len(checksum) != sha256.Size*2 {
		return "", "", errors.New("invalid managed sample checksum")
	}
	assets := model.YucoreMediaTaskAssets(task)
	if len(assets) != 1 || assets[0].ManagedFileName != model.YucoreMediaSampleFileName(checksum) {
		return "", "", errors.New("invalid managed sample asset")
	}
	managedPath, err := yucoreMediaSafeUploadPath(ownerID, assets[0].ManagedFileName)
	if err != nil {
		return "", "", errors.New("invalid managed sample path")
	}
	return checksum, managedPath, nil
}

func writeYucoreMediaSampleImportSuccess(c *gin.Context, task *model.YucoreMediaTask, checksum string, size int64, created bool) {
	common.ApiSuccess(c, gin.H{
		"created":   created,
		"task_id":   task.TaskId,
		"asset_url": fmt.Sprintf("/api/yucore/media/tasks/%s/assets/0", task.TaskId),
		"sha256":    checksum,
		"size":      size,
	})
}
