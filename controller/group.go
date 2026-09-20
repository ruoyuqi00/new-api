package controller

import (
	"net/http"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

type groupCatalogItem struct {
	Name               string   `json:"name"`
	Ratio              float64  `json:"ratio"`
	Public             bool     `json:"public"`
	Description        string   `json:"description"`
	ActiveChannelCount int      `json:"active_channel_count"`
	ActiveModelCount   int      `json:"active_model_count"`
	ActiveModels       []string `json:"active_models"`
}

type UserGroupInfo struct {
	Ratio any    `json:"ratio"`
	Desc  string `json:"desc"`
	service.GroupProtocolMetadata
}

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetGroupCatalog(c *gin.Context) {
	coverageByGroup, err := model.GetActiveGroupRoutingCoverage()
	if err != nil {
		common.SysLog("GetGroupCatalog routing coverage error: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": common.TranslateMessage(c, i18n.MsgDatabaseError),
		})
		return
	}

	ratioByGroup := ratio_setting.GetGroupRatioCopy()
	publicGroups := setting.GetUserUsableGroupsCopy()
	groupNames := make([]string, 0, len(ratioByGroup))
	for groupName := range ratioByGroup {
		groupNames = append(groupNames, groupName)
	}
	sort.Strings(groupNames)

	catalog := make([]groupCatalogItem, 0, len(groupNames))
	for _, groupName := range groupNames {
		description, public := publicGroups[groupName]
		coverage, hasCoverage := coverageByGroup[groupName]
		if !hasCoverage {
			coverage.ActiveModels = make([]string, 0)
		}
		catalog = append(catalog, groupCatalogItem{
			Name:               groupName,
			Ratio:              ratioByGroup[groupName],
			Public:             public,
			Description:        description,
			ActiveChannelCount: coverage.ActiveChannelCount,
			ActiveModelCount:   coverage.ActiveModelCount,
			ActiveModels:       coverage.ActiveModels,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    catalog,
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]UserGroupInfo)
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	visibleGroups := make([]string, 0, len(userUsableGroups))
	for groupName := range userUsableGroups {
		if groupName != "auto" {
			visibleGroups = append(visibleGroups, groupName)
		}
	}
	sort.Strings(visibleGroups)
	groupProtocolMetadata := make(map[string]service.GroupProtocolMetadata)
	if abilities, err := model.GetActiveAbilitiesForGroups(visibleGroups); err != nil {
		common.SysLog("failed to load group protocol metadata: " + err.Error())
	} else {
		groupProtocolMetadata = service.BuildGroupProtocolMetadata(abilities)
	}
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			metadata := groupProtocolMetadata[groupName]
			if metadata.Protocols == nil {
				metadata.Protocols = []string{}
			}
			if metadata.EndpointPaths == nil {
				metadata.EndpointPaths = []string{}
			}
			usableGroups[groupName] = UserGroupInfo{
				Ratio:                 service.GetUserGroupRatioForUser(userId, userGroup, groupName),
				Desc:                  desc,
				GroupProtocolMetadata: metadata,
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		metadata := service.MergeGroupProtocolMetadata(
			service.GetUserAutoGroup(userGroup),
			groupProtocolMetadata,
		)
		usableGroups["auto"] = UserGroupInfo{
			Ratio:                 "自动",
			Desc:                  setting.GetUsableGroupDescription("auto"),
			GroupProtocolMetadata: metadata,
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
