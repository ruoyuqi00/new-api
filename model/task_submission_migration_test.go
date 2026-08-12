package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTaskSubmissionKeyMigrationAddsColumnsBeforeUniqueIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:task_submission_migration?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE logs (id INTEGER PRIMARY KEY)").Error)
	require.NoError(t, db.Exec("CREATE TABLE tasks (id INTEGER PRIMARY KEY)").Error)

	require.NoError(t, migrateTaskSubmissionKeyColumns(db, true, true))
	require.NoError(t, db.AutoMigrate(&Log{}, &Task{}))
	require.NoError(t, ensureTaskSubmissionKeyIndexes(db, true, true))

	migrator := db.Migrator()
	assert.True(t, migrator.HasColumn(&Log{}, "SubmissionKey"))
	assert.True(t, migrator.HasColumn(&Task{}, "SubmissionKey"))
	assert.True(t, migrator.HasIndex(&logSubmissionKeyIndex{}, "idx_logs_submission_key"))
	assert.True(t, migrator.HasIndex(&taskSubmissionKeyIndex{}, "idx_tasks_submission_key"))

	firstKey := "submission_once"
	require.NoError(t, db.Create(&Log{SubmissionKey: &firstKey}).Error)
	require.Error(t, db.Create(&Log{SubmissionKey: &firstKey}).Error)
	require.NoError(t, db.Create(&Task{SubmissionKey: &firstKey, TaskID: "first"}).Error)
	require.Error(t, db.Create(&Task{SubmissionKey: &firstKey, TaskID: "second"}).Error)

	require.NoError(t, db.Create(&Log{}).Error)
	require.NoError(t, db.Create(&Log{}).Error)
	require.NoError(t, db.Create(&Task{TaskID: "legacy-one"}).Error)
	require.NoError(t, db.Create(&Task{TaskID: "legacy-two"}).Error)
}
