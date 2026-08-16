package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type yucoreMediaSampleImportResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Created  bool   `json:"created"`
		TaskID   string `json:"task_id"`
		AssetURL string `json:"asset_url"`
		SHA256   string `json:"sha256"`
		Size     int64  `json:"size"`
	} `json:"data"`
}

func setupYucoreMediaSampleControllerTest(t *testing.T) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	uploadRoot := t.TempDir()
	t.Setenv("YUCORE_MEDIA_UPLOAD_DIR", uploadRoot)

	originalDB := model.DB
	dsn := fmt.Sprintf("file:%x?mode=memory&cache=shared", sha256.Sum256([]byte(t.Name())))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.YucoreMediaTask{}))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{
		"yucore_media.model_capabilities": `{
			"sample-test-video":{"kind":"video","availability":"enabled"},
			"sample-test-image":{"kind":"image","availability":"enabled"},
			"sample-test-probe":{"kind":"video","availability":"probe"}
		}`,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})
	return uploadRoot
}

func performYucoreMediaSampleImport(
	t *testing.T,
	role int,
	content []byte,
	declaredMIME string,
	modelID string,
	checksum string,
	collectionID string,
	extraFields map[string]string,
) (*httptest.ResponseRecorder, yucoreMediaSampleImportResponse) {
	t.Helper()
	request := newYucoreMediaSampleImportRequest(t, content, declaredMIME, modelID, checksum, collectionID, extraFields)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Set("id", 42)
	context.Set("role", role)
	ImportYucoreMediaSample(context)

	var response yucoreMediaSampleImportResponse
	if recorder.Body.Len() > 0 {
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	}
	return recorder, response
}

func newYucoreMediaSampleImportRequest(
	t *testing.T,
	content []byte,
	declaredMIME string,
	modelID string,
	checksum string,
	collectionID string,
	extraFields map[string]string,
) *http.Request {
	t.Helper()
	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="sample.mp4"`)
	header.Set("Content-Type", declaredMIME)
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	for key, value := range map[string]string{
		"model_id":      modelID,
		"sha256":        checksum,
		"collection_id": collectionID,
	} {
		require.NoError(t, writer.WriteField(key, value))
	}
	for key, value := range extraFields {
		require.NoError(t, writer.WriteField(key, value))
	}
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, "/api/yucore/media/admin/sample-assets", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func yucoreMediaSampleChecksum(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func TestImportYucoreMediaSampleCreatesZeroCostCompletedTask(t *testing.T) {
	uploadRoot := setupYucoreMediaSampleControllerTest(t)
	content := yucoreMediaTestFTYP("isom", "mp41")
	checksum := yucoreMediaSampleChecksum(content)

	recorder, response := performYucoreMediaSampleImport(
		t, common.RoleAdminUser, content, "video/mp4", "sample-test-video", checksum,
		model.YucoreMediaSampleCollectionID, nil,
	)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, response.Success, response.Message)
	assert.True(t, response.Data.Created)
	assert.Equal(t, checksum, response.Data.SHA256)
	assert.Equal(t, int64(len(content)), response.Data.Size)
	assert.Equal(t, "/api/yucore/media/tasks/"+response.Data.TaskID+"/assets/0", response.Data.AssetURL)

	var task model.YucoreMediaTask
	require.NoError(t, model.DB.Where("task_id = ?", response.Data.TaskID).First(&task).Error)
	assert.Equal(t, model.YucoreMediaTaskStatusCompleted, task.Status)
	assert.Equal(t, 100, task.Progress)
	assert.Zero(t, task.Cost)
	assert.Equal(t, "video", task.Kind)
	assert.Equal(t, "sample-test-video", task.ModelId)
	assert.True(t, model.IsYucoreMediaSampleTask(&task))

	assets := model.YucoreMediaTaskAssets(&task)
	require.Len(t, assets, 1)
	managedPath := filepath.Join(uploadRoot, "42", assets[0].ManagedFileName)
	stored, err := os.ReadFile(managedPath)
	require.NoError(t, err)
	assert.Equal(t, content, stored)
}

func TestImportYucoreMediaSampleIsIdempotentAndConcurrentSafe(t *testing.T) {
	uploadRoot := setupYucoreMediaSampleControllerTest(t)
	content := yucoreMediaTestFTYP("isom", "mp41")
	checksum := yucoreMediaSampleChecksum(content)

	const concurrentImports = 2
	type pendingImport struct {
		context  *gin.Context
		recorder *httptest.ResponseRecorder
	}
	pending := make([]pendingImport, 0, concurrentImports)
	for range concurrentImports {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = newYucoreMediaSampleImportRequest(
			t, content, "video/mp4", "sample-test-video", checksum,
			model.YucoreMediaSampleCollectionID, nil,
		)
		context.Set("id", 42)
		context.Set("role", common.RoleAdminUser)
		pending = append(pending, pendingImport{context: context, recorder: recorder})
	}
	var waitGroup sync.WaitGroup
	for _, request := range pending {
		waitGroup.Add(1)
		go func(request pendingImport) {
			defer waitGroup.Done()
			ImportYucoreMediaSample(request.context)
		}(request)
	}
	waitGroup.Wait()
	createdCount := 0
	var taskID string
	for _, request := range pending {
		var response yucoreMediaSampleImportResponse
		require.NoError(t, common.Unmarshal(request.recorder.Body.Bytes(), &response))
		require.True(t, response.Success, response.Message)
		if response.Data.Created {
			createdCount++
		}
		if taskID == "" {
			taskID = response.Data.TaskID
		}
		assert.Equal(t, taskID, response.Data.TaskID)
	}
	assert.Equal(t, 1, createdCount)

	_, repeated := performYucoreMediaSampleImport(
		t, common.RoleAdminUser, content, "video/mp4", "sample-test-video", checksum,
		model.YucoreMediaSampleCollectionID, nil,
	)
	require.True(t, repeated.Success, repeated.Message)
	assert.False(t, repeated.Data.Created)
	assert.Equal(t, taskID, repeated.Data.TaskID)

	var count int64
	require.NoError(t, model.DB.Model(&model.YucoreMediaTask{}).Where("task_id = ?", taskID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	files, err := os.ReadDir(filepath.Join(uploadRoot, "42"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, model.YucoreMediaSampleFileName(checksum), files[0].Name())
}

func TestImportYucoreMediaSampleRejectsInvalidRequests(t *testing.T) {
	uploadRoot := setupYucoreMediaSampleControllerTest(t)
	content := yucoreMediaTestFTYP("isom", "mp41")
	checksum := yucoreMediaSampleChecksum(content)
	tests := []struct {
		name         string
		role         int
		content      []byte
		declaredMIME string
		modelID      string
		checksum     string
		collectionID string
		extraFields  map[string]string
		status       int
	}{
		{name: "non-admin", role: common.RoleCommonUser, content: content, declaredMIME: "video/mp4", modelID: "sample-test-video", checksum: checksum, collectionID: model.YucoreMediaSampleCollectionID, status: http.StatusForbidden},
		{name: "wrong collection", role: common.RoleAdminUser, content: content, declaredMIME: "video/mp4", modelID: "sample-test-video", checksum: checksum, collectionID: "other", status: http.StatusOK},
		{name: "unknown model", role: common.RoleAdminUser, content: content, declaredMIME: "video/mp4", modelID: "not-configured", checksum: checksum, collectionID: model.YucoreMediaSampleCollectionID, status: http.StatusOK},
		{name: "image model", role: common.RoleAdminUser, content: content, declaredMIME: "video/mp4", modelID: "sample-test-image", checksum: checksum, collectionID: model.YucoreMediaSampleCollectionID, status: http.StatusOK},
		{name: "probe model", role: common.RoleAdminUser, content: content, declaredMIME: "video/mp4", modelID: "sample-test-probe", checksum: checksum, collectionID: model.YucoreMediaSampleCollectionID, status: http.StatusOK},
		{name: "malformed checksum", role: common.RoleAdminUser, content: content, declaredMIME: "video/mp4", modelID: "sample-test-video", checksum: strings.Repeat("z", 64), collectionID: model.YucoreMediaSampleCollectionID, status: http.StatusOK},
		{name: "checksum mismatch", role: common.RoleAdminUser, content: content, declaredMIME: "video/mp4", modelID: "sample-test-video", checksum: strings.Repeat("a", 64), collectionID: model.YucoreMediaSampleCollectionID, status: http.StatusOK},
		{name: "spoofed MIME", role: common.RoleAdminUser, content: content, declaredMIME: "image/png", modelID: "sample-test-video", checksum: checksum, collectionID: model.YucoreMediaSampleCollectionID, status: http.StatusOK},
		{name: "unknown form field", role: common.RoleAdminUser, content: content, declaredMIME: "video/mp4", modelID: "sample-test-video", checksum: checksum, collectionID: model.YucoreMediaSampleCollectionID, extraFields: map[string]string{"unexpected": "value"}, status: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder, response := performYucoreMediaSampleImport(
				t, test.role, test.content, test.declaredMIME, test.modelID, test.checksum,
				test.collectionID, test.extraFields,
			)
			assert.Equal(t, test.status, recorder.Code)
			assert.False(t, response.Success)
			assert.NotEmpty(t, response.Message)
		})
	}

	var count int64
	require.NoError(t, model.DB.Model(&model.YucoreMediaTask{}).Count(&count).Error)
	assert.Zero(t, count)
	require.NoError(t, filepath.WalkDir(uploadRoot, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			t.Errorf("invalid sample request left file %s", entry.Name())
		}
		return nil
	}))
}

func TestImportYucoreMediaSampleCleansFileAfterDatabaseFailure(t *testing.T) {
	uploadRoot := setupYucoreMediaSampleControllerTest(t)
	content := yucoreMediaTestFTYP("isom", "mp41")
	checksum := yucoreMediaSampleChecksum(content)
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register("test:fail_sample_create", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Model.(*model.YucoreMediaTask); ok {
			tx.AddError(errors.New("injected sample create failure"))
		}
	}))

	_, response := performYucoreMediaSampleImport(
		t, common.RoleAdminUser, content, "video/mp4", "sample-test-video", checksum,
		model.YucoreMediaSampleCollectionID, nil,
	)
	assert.False(t, response.Success)
	_, err := os.Stat(filepath.Join(uploadRoot, "42", model.YucoreMediaSampleFileName(checksum)))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestImportYucoreMediaSampleRejectsExistingTaskWithoutManagedFile(t *testing.T) {
	setupYucoreMediaSampleControllerTest(t)
	content := yucoreMediaTestFTYP("isom", "mp41")
	checksum := yucoreMediaSampleChecksum(content)
	task, err := model.CreateYucoreMediaSampleTask(42, "sample-test-video", checksum, int64(len(content)))
	require.NoError(t, err)

	_, response := performYucoreMediaSampleImport(
		t, common.RoleAdminUser, content, "video/mp4", "sample-test-video", checksum,
		model.YucoreMediaSampleCollectionID, nil,
	)
	assert.False(t, response.Success)
	assert.Contains(t, strings.ToLower(response.Message), "conflict")
	var count int64
	require.NoError(t, model.DB.Model(&model.YucoreMediaTask{}).Where("task_id = ?", task.TaskId).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestDeleteYucoreMediaSampleRollsBackExactManagedAsset(t *testing.T) {
	uploadRoot := setupYucoreMediaSampleControllerTest(t)
	content := yucoreMediaTestFTYP("isom", "mp41")
	checksum := yucoreMediaSampleChecksum(content)
	_, imported := performYucoreMediaSampleImport(
		t, common.RoleAdminUser, content, "video/mp4", "sample-test-video", checksum,
		model.YucoreMediaSampleCollectionID, nil,
	)
	require.True(t, imported.Success, imported.Message)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodDelete, "/api/yucore/media/admin/sample-assets/"+imported.Data.TaskID, nil)
	context.Params = gin.Params{{Key: "task_id", Value: imported.Data.TaskID}}
	context.Set("id", 42)
	context.Set("role", common.RoleAdminUser)
	DeleteYucoreMediaSample(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)
	var count int64
	require.NoError(t, model.DB.Model(&model.YucoreMediaTask{}).Where("task_id = ?", imported.Data.TaskID).Count(&count).Error)
	assert.Zero(t, count)
	_, err := os.Stat(filepath.Join(uploadRoot, "42", model.YucoreMediaSampleFileName(checksum)))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestDeleteYucoreMediaSampleRestoresFileWhenDatabaseDeleteFails(t *testing.T) {
	uploadRoot := setupYucoreMediaSampleControllerTest(t)
	content := yucoreMediaTestFTYP("isom", "mp41")
	checksum := yucoreMediaSampleChecksum(content)
	_, imported := performYucoreMediaSampleImport(
		t, common.RoleAdminUser, content, "video/mp4", "sample-test-video", checksum,
		model.YucoreMediaSampleCollectionID, nil,
	)
	require.True(t, imported.Success, imported.Message)
	require.NoError(t, model.DB.Callback().Delete().Before("gorm:delete").Register("test:fail_sample_delete", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Model.(*model.YucoreMediaTask); ok {
			tx.AddError(errors.New("injected sample delete failure"))
		}
	}))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodDelete, "/api/yucore/media/admin/sample-assets/"+imported.Data.TaskID, nil)
	context.Params = gin.Params{{Key: "task_id", Value: imported.Data.TaskID}}
	context.Set("id", 42)
	context.Set("role", common.RoleAdminUser)
	DeleteYucoreMediaSample(context)

	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	var count int64
	require.NoError(t, model.DB.Model(&model.YucoreMediaTask{}).Where("task_id = ?", imported.Data.TaskID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	restored, err := os.ReadFile(filepath.Join(uploadRoot, "42", model.YucoreMediaSampleFileName(checksum)))
	require.NoError(t, err)
	assert.Equal(t, content, restored)
}

func TestDeleteYucoreMediaSampleRejectsOrdinaryTask(t *testing.T) {
	setupYucoreMediaSampleControllerTest(t)
	ordinary := &model.YucoreMediaTask{
		TaskId: "ordinary-task", UserId: 42, Kind: "video", Mode: "text-to-video",
		ModelId: "sample-test-video", Status: model.YucoreMediaTaskStatusCompleted,
	}
	require.NoError(t, model.DB.Create(ordinary).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodDelete, "/api/yucore/media/admin/sample-assets/ordinary-task", nil)
	context.Params = gin.Params{{Key: "task_id", Value: ordinary.TaskId}}
	context.Set("id", 42)
	context.Set("role", common.RoleAdminUser)
	DeleteYucoreMediaSample(context)

	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	var count int64
	require.NoError(t, model.DB.Model(&model.YucoreMediaTask{}).Where("task_id = ?", ordinary.TaskId).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
