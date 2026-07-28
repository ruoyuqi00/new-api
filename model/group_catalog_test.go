package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupGroupCatalogModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	initCol()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
	DB = db
	LOG_DB = db

	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		initCol()
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestGetActiveGroupRoutingCoverageDeduplicatesChannelsAndModels(t *testing.T) {
	db := setupGroupCatalogModelTestDB(t)
	require.NoError(t, db.Create(&[]Channel{
		{Id: 1, Name: "private-one", Key: "key-1", Group: "private", Models: "image-model,video-model", Status: common.ChannelStatusEnabled},
		{Id: 2, Name: "private-two", Key: "key-2", Group: "private", Models: "image-model", Status: common.ChannelStatusEnabled},
		{Id: 3, Name: "disabled-channel", Key: "key-3", Group: "private", Models: "disabled-channel-model", Status: common.ChannelStatusManuallyDisabled},
		{Id: 4, Name: "disabled-ability", Key: "key-4", Group: "private", Models: "disabled-ability-model", Status: common.ChannelStatusEnabled},
		{Id: 5, Name: "public", Key: "key-5", Group: "public", Models: "public-model", Status: common.ChannelStatusEnabled},
	}).Error)
	require.NoError(t, db.Create(&[]Ability{
		{Group: "private", Model: "video-model", ChannelId: 1, Enabled: true},
		{Group: "private", Model: "image-model", ChannelId: 1, Enabled: true},
		{Group: "private", Model: "image-model", ChannelId: 2, Enabled: true},
		{Group: "private", Model: "disabled-channel-model", ChannelId: 3, Enabled: true},
		{Group: "private", Model: "disabled-ability-model", ChannelId: 4, Enabled: false},
		{Group: "public", Model: "public-model", ChannelId: 5, Enabled: true},
	}).Error)

	coverage, err := GetActiveGroupRoutingCoverage()
	require.NoError(t, err)

	require.Contains(t, coverage, "private")
	assert.Equal(t, 2, coverage["private"].ActiveChannelCount)
	assert.Equal(t, 2, coverage["private"].ActiveModelCount)
	assert.Equal(t, []string{"image-model", "video-model"}, coverage["private"].ActiveModels)

	require.Contains(t, coverage, "public")
	assert.Equal(t, 1, coverage["public"].ActiveChannelCount)
	assert.Equal(t, 1, coverage["public"].ActiveModelCount)
	assert.Equal(t, []string{"public-model"}, coverage["public"].ActiveModels)
}
