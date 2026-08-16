package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestYucoreMediaSampleIdentityIsOwnerScopedAndDeterministic(t *testing.T) {
	checksum := strings.Repeat("a", 64)
	firstID, err := YucoreMediaSampleTaskID(42, checksum)
	require.NoError(t, err)
	secondID, err := YucoreMediaSampleTaskID(42, strings.ToUpper(checksum))
	require.NoError(t, err)
	otherOwnerID, err := YucoreMediaSampleTaskID(7, checksum)
	require.NoError(t, err)

	assert.Equal(t, firstID, secondID)
	assert.NotEqual(t, firstID, otherOwnerID)
	assert.LessOrEqual(t, len(firstID), 64)
	assert.Equal(t, "sample_"+checksum+".mp4", YucoreMediaSampleFileName(checksum))
}

func TestYucoreMediaSampleIdentityRejectsInvalidInput(t *testing.T) {
	validChecksum := strings.Repeat("a", 64)
	for _, test := range []struct {
		name          string
		userID        int
		checksum      string
		validFileName bool
	}{
		{name: "zero owner", userID: 0, checksum: validChecksum, validFileName: true},
		{name: "negative owner", userID: -1, checksum: validChecksum, validFileName: true},
		{name: "short checksum", userID: 42, checksum: strings.Repeat("a", 63)},
		{name: "long checksum", userID: 42, checksum: strings.Repeat("a", 65)},
		{name: "non hex checksum", userID: 42, checksum: strings.Repeat("g", 64)},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := YucoreMediaSampleTaskID(test.userID, test.checksum)
			assert.Error(t, err)
			if test.validFileName {
				assert.NotEmpty(t, YucoreMediaSampleFileName(test.checksum))
			} else {
				assert.Empty(t, YucoreMediaSampleFileName(test.checksum))
			}
		})
	}
}

func TestYucoreMediaSampleManagedFileStaysOutOfPublicJSON(t *testing.T) {
	managedFileName := "sample_" + strings.Repeat("b", 64) + ".mp4"
	assets := []YucoreMediaAsset{{
		Id:              "sample_asset",
		Kind:            "video",
		Url:             "/api/yucore/media/tasks/id/assets/0",
		ManagedFileName: managedFileName,
	}}
	raw, err := marshalYucoreMediaAssets(assets)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"managed_file_name"`)
	assert.Contains(t, string(raw), managedFileName)

	task := &YucoreMediaTask{Assets: YucoreMediaAssets(raw)}
	publicAssets := YucoreMediaTaskAssets(task)
	require.Len(t, publicAssets, 1)
	assert.Equal(t, managedFileName, publicAssets[0].ManagedFileName)
	encoded, err := common.Marshal(publicAssets)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "managed_file")
	assert.NotContains(t, string(encoded), managedFileName)
}

func TestYucoreMediaSampleDetectionRejectsSpoofedTasks(t *testing.T) {
	checksum := strings.Repeat("c", 64)
	taskID, err := YucoreMediaSampleTaskID(42, checksum)
	require.NoError(t, err)
	valid := &YucoreMediaTask{
		TaskId: taskID,
		UserId: 42,
		Mode:   YucoreMediaSampleMode,
		Metadata: mergeYucoreMediaMetadata("", map[string]any{
			"imported_sample": true,
			"collection_id":   YucoreMediaSampleCollectionID,
			"sha256":          checksum,
		}),
	}
	assert.True(t, IsYucoreMediaSampleTask(valid))

	for _, mutate := range []func(*YucoreMediaTask){
		func(task *YucoreMediaTask) { task.TaskId = "yu_regular" },
		func(task *YucoreMediaTask) { task.UserId = 7 },
		func(task *YucoreMediaTask) { task.Mode = "text-to-video" },
		func(task *YucoreMediaTask) {
			task.Metadata = `{"imported_sample":true,"collection_id":"video-model-examples"}`
		},
		func(task *YucoreMediaTask) {
			task.Metadata = `{"imported_sample":true,"collection_id":"other","sha256":"` + checksum + `"}`
		},
		func(task *YucoreMediaTask) {
			task.Metadata = `{"imported_sample":"true","collection_id":"video-model-examples","sha256":"` + checksum + `"}`
		},
		func(task *YucoreMediaTask) { task.Metadata = `{}`; task.Prompt = YucoreMediaSampleCollectionName },
	} {
		candidate := *valid
		mutate(&candidate)
		assert.False(t, IsYucoreMediaSampleTask(&candidate))
	}
}

func TestYucoreMediaSampleTaskPersistsCompletedWithoutBilling(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:yucore_media_sample_create?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&YucoreMediaTask{}))
	DB = db
	t.Cleanup(func() { DB = originalDB })

	checksum := strings.Repeat("d", 64)
	task, err := CreateYucoreMediaSampleTask(42, "seedance-2.0", checksum, 4096)
	require.NoError(t, err)
	require.NotNil(t, task)

	var stored YucoreMediaTask
	require.NoError(t, db.Where("task_id = ?", task.TaskId).First(&stored).Error)
	assert.Equal(t, YucoreMediaTaskStatusCompleted, stored.Status)
	assert.Equal(t, 100, stored.Progress)
	assert.Zero(t, stored.Cost)
	assert.Equal(t, "video", stored.Kind)
	assert.Equal(t, YucoreMediaSampleMode, stored.Mode)
	assert.Equal(t, "seedance-2.0", stored.ModelId)
	assert.True(t, IsYucoreMediaSampleTask(&stored))
	assert.Positive(t, stored.CreatedTime)
	assert.Equal(t, stored.CreatedTime, stored.UpdatedTime)

	assets := YucoreMediaTaskAssets(&stored)
	require.Len(t, assets, 1)
	assert.Equal(t, "video/mp4", assets[0].MimeType)
	assert.Equal(t, YucoreMediaSampleFileName(checksum), assets[0].ManagedFileName)
	assert.Equal(t, "/api/yucore/media/tasks/"+stored.TaskId+"/assets/0", assets[0].Url)

	metadata := yucoreMediaMetadataMap(stored.Metadata)
	assert.Equal(t, true, metadata["imported_sample"])
	assert.Equal(t, YucoreMediaSampleCollectionID, metadata["collection_id"])
	assert.Equal(t, checksum, metadata["sha256"])
	assert.Equal(t, float64(4096), metadata["size"])
}

func TestYucoreMediaSampleQueryAccessExcludesSamplesFromRowsAndTotals(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:yucore_media_sample_query_access?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&YucoreMediaTask{}))
	DB = db
	t.Cleanup(func() { DB = originalDB })

	checksum := strings.Repeat("e", 64)
	sample, err := CreateYucoreMediaSampleTask(42, "seedance-2.0", checksum, 4096)
	require.NoError(t, err)
	ordinary := &YucoreMediaTask{
		TaskId: "yu_ordinary_query_task", UserId: 42, Kind: "video", Mode: "text-to-video",
		ModelId: "seedance-2.0", Status: YucoreMediaTaskStatusCompleted,
	}
	wildcardLookalike := &YucoreMediaTask{
		TaskId: "yuxsampleynot-managed", UserId: 42, Kind: "video", Mode: "text-to-video",
		ModelId: "seedance-2.0", Status: YucoreMediaTaskStatusCompleted,
	}
	require.NoError(t, db.Create(ordinary).Error)
	require.NoError(t, db.Create(wildcardLookalike).Error)

	visible, err := ListYucoreMediaTasks(42, "", "video", YucoreMediaTaskStatusCompleted, 0, 100, false)
	require.NoError(t, err)
	require.Len(t, visible, 2)
	assert.ElementsMatch(t, []string{ordinary.TaskId, wildcardLookalike.TaskId}, []string{visible[0].TaskId, visible[1].TaskId})
	visibleTotal, err := CountYucoreMediaTasks(42, "", "video", YucoreMediaTaskStatusCompleted, false)
	require.NoError(t, err)
	assert.Equal(t, int64(2), visibleTotal)

	withSamples, err := ListYucoreMediaTasks(42, "", "video", YucoreMediaTaskStatusCompleted, 0, 100, true)
	require.NoError(t, err)
	require.Len(t, withSamples, 3)
	assert.ElementsMatch(t, []string{ordinary.TaskId, wildcardLookalike.TaskId, sample.TaskId}, []string{withSamples[0].TaskId, withSamples[1].TaskId, withSamples[2].TaskId})
	withSamplesTotal, err := CountYucoreMediaTasks(42, "", "video", YucoreMediaTaskStatusCompleted, true)
	require.NoError(t, err)
	assert.Equal(t, int64(3), withSamplesTotal)
}
