package ratio_setting

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

var defaultGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var groupRatioMap = types.NewRWMap[string, float64]()

var defaultGroupGroupRatio = map[string]map[string]float64{
	"vip": {
		"edit_this": 0.9,
	},
}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

var defaultGroupSpecialUsableGroup = map[string]map[string]string{}
var defaultUserGroupRatio = map[string]map[string]float64{}
var defaultAvailabilityMonitoring = map[string]bool{}

type GroupRatioSetting struct {
	GroupRatio              *types.RWMap[string, float64]            `json:"group_ratio"`
	GroupGroupRatio         *types.RWMap[string, map[string]float64] `json:"group_group_ratio"`
	GroupSpecialUsableGroup *types.RWMap[string, map[string]string]  `json:"group_special_usable_group"`
	UserGroupRatio          *types.RWMap[string, map[string]float64] `json:"user_group_ratio"`
	AvailabilityMonitoring  *types.RWMap[string, bool]               `json:"availability_monitoring"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	userGroupRatio := types.NewRWMap[string, map[string]float64]()
	userGroupRatio.AddAll(defaultUserGroupRatio)
	availabilityMonitoring := types.NewRWMap[string, bool]()
	availabilityMonitoring.AddAll(defaultAvailabilityMonitoring)

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup: groupSpecialUsableGroup,
		GroupRatio:              groupRatioMap,
		GroupGroupRatio:         groupGroupRatioMap,
		UserGroupRatio:          userGroupRatio,
		AvailabilityMonitoring:  availabilityMonitoring,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting() *GroupRatioSetting {
	if groupRatioSetting.GroupSpecialUsableGroup == nil {
		groupRatioSetting.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
		groupRatioSetting.GroupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	}
	if groupRatioSetting.UserGroupRatio == nil {
		groupRatioSetting.UserGroupRatio = types.NewRWMap[string, map[string]float64]()
		groupRatioSetting.UserGroupRatio.AddAll(defaultUserGroupRatio)
	}
	if groupRatioSetting.AvailabilityMonitoring == nil {
		groupRatioSetting.AvailabilityMonitoring = types.NewRWMap[string, bool]()
		groupRatioSetting.AvailabilityMonitoring.AddAll(defaultAvailabilityMonitoring)
	}
	return &groupRatioSetting
}

func GetGroupRatioCopy() map[string]float64 {
	return groupRatioMap.ReadAll()
}

func ContainsGroupRatio(name string) bool {
	_, ok := groupRatioMap.Get(name)
	return ok
}

func GroupRatio2JSONString() string {
	return groupRatioMap.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupRatioMap, jsonStr)
}

func GetGroupRatio(name string) float64 {
	ratio, ok := groupRatioMap.Get(name)
	if !ok {
		common.SysLog("group ratio not found: " + name)
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(userGroup, usingGroup string) (float64, bool) {
	gp, ok := groupGroupRatioMap.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok {
		return -1, false
	}
	return ratio, true
}

// GetUserGroupRatio resolves an individual user's override before the
// existing user-group override. The boolean reports whether either override
// was found; callers can fall back to the base group ratio when false.
func GetUserGroupRatio(userID int, userGroup, usingGroup string) (float64, bool) {
	if userID > 0 {
		if overrides, ok := groupRatioSetting.UserGroupRatio.Get(strconv.Itoa(userID)); ok {
			if ratio, exists := overrides[usingGroup]; exists {
				return ratio, true
			}
		}
	}
	return GetGroupGroupRatio(userGroup, usingGroup)
}

func UserGroupRatio2JSONString() string {
	return groupRatioSetting.UserGroupRatio.MarshalJSONString()
}

func UpdateUserGroupRatioByJSONString(jsonStr string) error {
	if err := CheckUserGroupRatio(jsonStr); err != nil {
		return err
	}
	return types.LoadFromJsonString(groupRatioSetting.UserGroupRatio, jsonStr)
}

func CheckUserGroupRatio(jsonStr string) error {
	values := make(map[string]map[string]*float64)
	if err := common.Unmarshal([]byte(jsonStr), &values); err != nil {
		return err
	}
	for userID, groupRatios := range values {
		if strings.TrimSpace(userID) == "" {
			return errors.New("user id must not be empty")
		}
		parsedID, err := strconv.Atoi(userID)
		if err != nil || parsedID <= 0 {
			return errors.New("user id must be a positive integer: " + userID)
		}
		for group, ratio := range groupRatios {
			if strings.TrimSpace(group) == "" {
				return errors.New("target group must not be empty")
			}
			if ratio == nil || *ratio < 0 || math.IsNaN(*ratio) || math.IsInf(*ratio, 0) {
				return errors.New("user group ratio must be finite and not less than 0: " + group)
			}
		}
	}
	return nil
}

func AvailabilityMonitoring2JSONString() string {
	return groupRatioSetting.AvailabilityMonitoring.MarshalJSONString()
}

func UpdateAvailabilityMonitoringByJSONString(jsonStr string) error {
	if err := CheckAvailabilityMonitoring(jsonStr); err != nil {
		return err
	}
	return types.LoadFromJsonString(groupRatioSetting.AvailabilityMonitoring, jsonStr)
}

func CheckAvailabilityMonitoring(jsonStr string) error {
	values := make(map[string]bool)
	if err := common.Unmarshal([]byte(jsonStr), &values); err != nil {
		return err
	}
	for group := range values {
		if strings.TrimSpace(group) == "" {
			return errors.New("monitoring group must not be empty")
		}
	}
	return nil
}

func IsAvailabilityMonitoringEnabled(group string) bool {
	enabled, ok := groupRatioSetting.AvailabilityMonitoring.Get(group)
	return ok && enabled
}

func GetAvailabilityMonitoringGroupsCopy() map[string]bool {
	return groupRatioSetting.AvailabilityMonitoring.ReadAll()
}

func GroupGroupRatio2JSONString() string {
	return groupGroupRatioMap.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupGroupRatioMap, jsonStr)
}

func CheckGroupRatio(jsonStr string) error {
	checkGroupRatio := make(map[string]float64)
	err := json.Unmarshal([]byte(jsonStr), &checkGroupRatio)
	if err != nil {
		return err
	}
	for name, ratio := range checkGroupRatio {
		if ratio < 0 {
			return errors.New("group ratio must be not less than 0: " + name)
		}
	}
	return nil
}
