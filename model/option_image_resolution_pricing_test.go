package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupImageResolutionOptionTest(t *testing.T) {
	t.Helper()
	originalDB := DB
	originalLogDB := LOG_DB
	originalOptions := common.OptionMap
	originalConfig := make(map[string]string)
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		if strings.HasPrefix(key, "image_resolution_price_setting.") {
			originalConfig[key] = value
		}
		return nil
	}))

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	DB = db
	LOG_DB = db
	common.OptionMapRWMutex.Lock()
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(originalConfig))
		operation_setting.RebuildImageResolutionPriceIndex()
		InvalidatePricingCache()
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
		DB = originalDB
		LOG_DB = originalLogDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
}

func TestUpdateOptionImageResolutionPricingValidatesBeforePersistence(t *testing.T) {
	setupImageResolutionOptionTest(t)
	valid := `{"gpt-image-2":{"prices":{"1k":0.02,"2k":0.04,"4k":0.08},"default_tier":"1k"}}`
	require.NoError(t, UpdateOption("image_resolution_price_setting.models", valid))

	var stored Option
	require.NoError(t, DB.First(&stored, "key = ?", "image_resolution_price_setting.models").Error)
	assert.JSONEq(t, valid, stored.Value)
	quote, configured, err := operation_setting.ResolveImageResolutionPrice("gpt-image-2", "1k")
	require.NoError(t, err)
	require.True(t, configured)
	assert.InDelta(t, 0.02, quote.UnitPrice, 1e-12)

	invalid := `{"gpt-image-2":{"prices":{"1k":0.02,"2k":0.01,"4k":0.08},"default_tier":"1k"}}`
	require.Error(t, UpdateOption("image_resolution_price_setting.models", invalid))
	require.NoError(t, DB.First(&stored, "key = ?", "image_resolution_price_setting.models").Error)
	assert.JSONEq(t, valid, stored.Value)
	quote, configured, err = operation_setting.ResolveImageResolutionPrice("gpt-image-2", "1k")
	require.NoError(t, err)
	require.True(t, configured)
	assert.InDelta(t, 0.02, quote.UnitPrice, 1e-12)
}

func TestUpdateOptionsBulkRejectsInvalidImageResolutionPricingBeforeTransaction(t *testing.T) {
	setupImageResolutionOptionTest(t)
	err := UpdateOptionsBulk(map[string]string{
		"SystemName":                            "must-not-commit",
		"image_resolution_price_setting.models": `{"gpt-image-2":{"prices":{"1k":0.01},"default_tier":"1k"}}`,
	})
	require.Error(t, err)

	var count int64
	require.NoError(t, DB.Model(&Option{}).Where("key = ? AND value = ?", "SystemName", "must-not-commit").Count(&count).Error)
	assert.Zero(t, count)
}
