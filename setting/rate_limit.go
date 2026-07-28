package setting

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var ModelRequestRateLimitEnabled = false
var ModelRequestRateLimitDurationMinutes = 1
var ModelRequestRateLimitCount = 0
var ModelRequestRateLimitSuccessCount = 1000
var ModelRequestRateLimitGroup = map[string][2]int{}
var ModelRequestRateLimitMutex sync.RWMutex
var UserConcurrencyLimitEnabled = true
var UserConcurrencyLimit = 5
var UserConcurrencyLimitGroup = map[string]int{}
var UserConcurrencyLimitMutex sync.RWMutex

func ModelRequestRateLimitGroup2JSONString() string {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := json.Marshal(ModelRequestRateLimitGroup)
	if err != nil {
		common.SysLog("error marshalling model ratio: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateModelRequestRateLimitGroupByJSONString(jsonStr string) error {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	ModelRequestRateLimitGroup = make(map[string][2]int)
	return json.Unmarshal([]byte(jsonStr), &ModelRequestRateLimitGroup)
}

func GetGroupRateLimit(group string) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	if ModelRequestRateLimitGroup == nil {
		return 0, 0, false
	}

	limits, found := ModelRequestRateLimitGroup[group]
	if !found {
		return 0, 0, false
	}
	return limits[0], limits[1], true
}

func CheckModelRequestRateLimitGroup(jsonStr string) error {
	checkModelRequestRateLimitGroup := make(map[string][2]int)
	err := json.Unmarshal([]byte(jsonStr), &checkModelRequestRateLimitGroup)
	if err != nil {
		return err
	}
	for group, limits := range checkModelRequestRateLimitGroup {
		if limits[0] < 0 || limits[1] < 1 {
			return fmt.Errorf("group %s has negative rate limit values: [%d, %d]", group, limits[0], limits[1])
		}
		if limits[0] > math.MaxInt32 || limits[1] > math.MaxInt32 {
			return fmt.Errorf("group %s [%d, %d] has max rate limits value 2147483647", group, limits[0], limits[1])
		}
	}

	return nil
}

func UserConcurrencyLimitGroup2JSONString() string {
	UserConcurrencyLimitMutex.RLock()
	defer UserConcurrencyLimitMutex.RUnlock()

	jsonBytes, err := json.Marshal(UserConcurrencyLimitGroup)
	if err != nil {
		common.SysLog("error marshalling user concurrency limit group: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateUserConcurrencyLimitGroupByJSONString(jsonStr string) error {
	UserConcurrencyLimitMutex.Lock()
	defer UserConcurrencyLimitMutex.Unlock()

	if jsonStr == "" {
		UserConcurrencyLimitGroup = map[string]int{}
		return nil
	}

	limits := make(map[string]int)
	if err := json.Unmarshal([]byte(jsonStr), &limits); err != nil {
		return err
	}
	UserConcurrencyLimitGroup = limits
	return nil
}

func GetGroupConcurrencyLimit(group string) (limit int, found bool) {
	UserConcurrencyLimitMutex.RLock()
	defer UserConcurrencyLimitMutex.RUnlock()

	if UserConcurrencyLimitGroup == nil {
		return 0, false
	}

	limit, found = UserConcurrencyLimitGroup[group]
	return limit, found
}

func CheckUserConcurrencyLimitGroup(jsonStr string) error {
	if jsonStr == "" {
		return nil
	}
	checkUserConcurrencyLimitGroup := make(map[string]int)
	err := json.Unmarshal([]byte(jsonStr), &checkUserConcurrencyLimitGroup)
	if err != nil {
		return err
	}
	for group, limit := range checkUserConcurrencyLimitGroup {
		if limit < 0 {
			return fmt.Errorf("group %s has negative concurrency limit: %d", group, limit)
		}
		if limit > math.MaxInt32 {
			return fmt.Errorf("group %s has max concurrency limit 2147483647", group)
		}
	}

	return nil
}
