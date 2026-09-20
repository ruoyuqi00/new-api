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

func setupVideoTierPricingOptionTest(t *testing.T) {
	t.Helper()
	originalDB := DB
	originalLogDB := LOG_DB
	originalOptions := common.OptionMap
	originalConfig := make(map[string]string)
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		if strings.HasPrefix(key, "video_pricing_setting.") {
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
		operation_setting.RebuildVideoTierPriceIndex()
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

func TestUpdateOptionVideoTierPricingValidatesBeforePersistence(t *testing.T) {
	setupVideoTierPricingOptionTest(t)
	valid := `{
		"minimax-h3": {
			"480p":{"standard":0.11},
			"768p":{"standard":0.17},
			"1080p":{"standard":0.21},
			"2k":{"standard":0.31},
			"4k":{"standard":0.47}
		}
	}`
	require.NoError(t, UpdateOption("video_pricing_setting.models", valid))

	var stored Option
	require.NoError(t, DB.First(&stored, "key = ?", "video_pricing_setting.models").Error)
	assert.JSONEq(t, valid, stored.Value)
	quote, configured, err := operation_setting.ResolveVideoTierPrice("minimax-h3", "4k", false, 0.10)
	require.NoError(t, err)
	require.True(t, configured)
	assert.InDelta(t, 0.47, quote.UnitPrice, 1e-12)

	invalid := `{"minimax-h3":{"480p":{"standard":0.11}}}`
	require.Error(t, UpdateOption("video_pricing_setting.models", invalid))
	require.NoError(t, DB.First(&stored, "key = ?", "video_pricing_setting.models").Error)
	assert.JSONEq(t, valid, stored.Value)
	quote, configured, err = operation_setting.ResolveVideoTierPrice("minimax-h3", "4k", false, 0.10)
	require.NoError(t, err)
	require.True(t, configured)
	assert.InDelta(t, 0.47, quote.UnitPrice, 1e-12)
}

func TestUpdateOptionsBulkRejectsInvalidVideoTierPricingBeforeTransaction(t *testing.T) {
	setupVideoTierPricingOptionTest(t)
	err := UpdateOptionsBulk(map[string]string{
		"SystemName":                   "must-not-commit",
		"video_pricing_setting.models": `{"minimax-h3":{"480p":{"standard":0.11}}}`,
	})
	require.Error(t, err)

	var count int64
	require.NoError(t, DB.Model(&Option{}).Where("key = ? AND value = ?", "SystemName", "must-not-commit").Count(&count).Error)
	assert.Zero(t, count)
}
