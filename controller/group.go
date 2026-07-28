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
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	for groupName, _ := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]interface{}{
				"ratio": service.GetUserGroupRatio(userGroup, groupName),
				"desc":  desc,
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": "自动",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
