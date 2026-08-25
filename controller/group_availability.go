package controller

import (
	"net/http"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/groupavailability"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type userGroupAvailabilityItem struct {
	Group        string  `json:"group"`
	Description  string  `json:"description"`
	RequestCount int64   `json:"request_count"`
	SuccessCount int64   `json:"success_count"`
	SuccessRate  float64 `json:"success_rate"`
	Status       string  `json:"status"`
	ObservedAt   int64   `json:"observed_at"`
}

func GetUserGroupAvailability(c *gin.Context) {
	userID := c.GetInt("id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "unauthorized"})
		return
	}
	user, err := model.GetUserById(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	usableGroups := service.GetUserUsableGroups(user.Group)
	monitoringGroups := ratio_setting.GetAvailabilityMonitoringGroupsCopy()
	groups := make([]string, 0, len(monitoringGroups))
	for group, enabled := range monitoringGroups {
		if enabled {
			if _, usable := usableGroups[group]; usable {
				groups = append(groups, group)
			}
		}
	}
	sort.Strings(groups)

	items := make([]userGroupAvailabilityItem, 0, len(groups))
	for _, group := range groups {
		summary, _ := groupavailability.Query(group)
		items = append(items, userGroupAvailabilityItem{
			Group:        group,
			Description:  usableGroups[group],
			RequestCount: summary.RequestCount,
			SuccessCount: summary.SuccessCount,
			SuccessRate:  summary.SuccessRate,
			Status:       summary.Status,
			ObservedAt:   summary.ObservedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
	})
}
