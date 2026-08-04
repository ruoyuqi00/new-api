package setting

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/types"
)

var CheckSensitiveEnabled = true
var CheckSensitiveOnPromptEnabled = true

//var CheckSensitiveOnCompletionEnabled = true

// StopOnSensitiveEnabled 如果检测到敏感词，是否立刻停止生成，否则替换敏感词
var StopOnSensitiveEnabled = true

// StreamCacheQueueLength 流模式缓存队列长度，0表示无缓存
var StreamCacheQueueLength = 0

// SensitiveWords 敏感词
// var SensitiveWords []string
var SensitiveWords = []string{
	"test_sensitive",
}

var SensitiveInputRetentionDays = 7

var sensitiveInputCheckGroups = types.NewRWMap[string, bool]()

func SensitiveWordsToString() string {
	return strings.Join(SensitiveWords, "\n")
}

func SensitiveWordsFromString(s string) {
	SensitiveWords = []string{}
	sw := strings.Split(s, "\n")
	for _, w := range sw {
		w = strings.TrimSpace(w)
		if w != "" {
			SensitiveWords = append(SensitiveWords, w)
		}
	}
}

func ParseSensitiveInputRetentionDays(value string) (int, error) {
	days, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || days < 1 || days > 365 {
		return 0, fmt.Errorf("sensitive input retention days must be between 1 and 365")
	}
	return days, nil
}

func UpdateSensitiveInputRetentionDays(value string) error {
	days, err := ParseSensitiveInputRetentionDays(value)
	if err != nil {
		return err
	}
	SensitiveInputRetentionDays = days
	return nil
}

func ShouldCheckPromptSensitive() bool {
	return CheckSensitiveEnabled && CheckSensitiveOnPromptEnabled
}

func SensitiveInputCheckGroups2JSONString() string {
	return sensitiveInputCheckGroups.MarshalJSONString()
}

func UpdateSensitiveInputCheckGroupsByJSONString(value string) error {
	return types.LoadFromJsonString(sensitiveInputCheckGroups, value)
}

func ShouldCheckPromptSensitiveForGroup(group string) bool {
	if !ShouldCheckPromptSensitive() {
		return false
	}
	enabled, exists := sensitiveInputCheckGroups.Get(group)
	return !exists || enabled
}

//func ShouldCheckCompletionSensitive() bool {
//	return CheckSensitiveEnabled && CheckSensitiveOnCompletionEnabled
//}
