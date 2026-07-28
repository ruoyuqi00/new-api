package model

import (
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/common"
)

type GroupRoutingCoverage struct {
	ActiveChannelCount int      `json:"active_channel_count"`
	ActiveModelCount   int      `json:"active_model_count"`
	ActiveModels       []string `json:"active_models"`
}

func GetActiveGroupRoutingCoverage() (map[string]GroupRoutingCoverage, error) {
	type routingRow struct {
		GroupName string `gorm:"column:group_name"`
		Model     string `gorm:"column:model"`
		ChannelID int    `gorm:"column:channel_id"`
	}

	var rows []routingRow
	groupColumn := "abilities." + commonGroupCol
	err := DB.Table("abilities").
		Select(fmt.Sprintf("%s AS group_name, abilities.model, abilities.channel_id", groupColumn)).
		Joins("JOIN channels ON channels.id = abilities.channel_id").
		Where("abilities.enabled = ? AND channels.status = ?", true, common.ChannelStatusEnabled).
		Distinct().
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	type coverageSets struct {
		channels map[int]struct{}
		models   map[string]struct{}
	}
	setsByGroup := make(map[string]*coverageSets)
	for _, row := range rows {
		sets := setsByGroup[row.GroupName]
		if sets == nil {
			sets = &coverageSets{
				channels: make(map[int]struct{}),
				models:   make(map[string]struct{}),
			}
			setsByGroup[row.GroupName] = sets
		}
		sets.channels[row.ChannelID] = struct{}{}
		sets.models[row.Model] = struct{}{}
	}

	coverageByGroup := make(map[string]GroupRoutingCoverage, len(setsByGroup))
	for groupName, sets := range setsByGroup {
		models := make([]string, 0, len(sets.models))
		for modelName := range sets.models {
			models = append(models, modelName)
		}
		sort.Strings(models)
		coverageByGroup[groupName] = GroupRoutingCoverage{
			ActiveChannelCount: len(sets.channels),
			ActiveModelCount:   len(models),
			ActiveModels:       models,
		}
	}

	return coverageByGroup, nil
}
