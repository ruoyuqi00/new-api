package model

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const (
	channelPoolCooldownNamespace    = "new-api:channel_pool:cooldown:v1"
	channelPoolConcurrencyNamespace = "new-api:channel_pool:concurrency:v1"
	channelPoolLeaseTTL             = 6 * time.Hour
)

var (
	channelPoolMemoryMu       sync.Mutex
	channelPoolMemoryInflight = map[int]int{}
	channelPoolMemoryCooldown = map[string]time.Time{}
)

type ChannelSelectionOptions struct {
	SkipChannelIDs map[int]struct{}
}

const (
	ChannelPoolCandidateReasonAvailable = "available"
	ChannelPoolCandidateReasonCooldown  = "cooldown"
	ChannelPoolCandidateReasonFull      = "full"
	ChannelPoolCandidateReasonNoChannel = "no_channel"
)

type ChannelPoolCandidateStatus struct {
	ChannelID    int
	Available    bool
	Reason       string
	Limit        int
	Inflight     int
	CoolingDown  bool
	HasHardLimit bool
}

type ChannelPoolSelectionSnapshot struct {
	CacheEnabled          bool
	CandidateCount        int
	AvailableCount        int
	FullCount             int
	CooldownCount         int
	MissingChannelCount   int
	SelectionSkippedCount int
	PathSkippedCount      int
}

type ChannelPoolLease struct {
	channelID int
	key       string
	redis     bool
	released  int32
}

func (l *ChannelPoolLease) ChannelID() int {
	if l == nil {
		return 0
	}
	return l.channelID
}

func (l *ChannelPoolLease) Release() {
	if l == nil || !atomic.CompareAndSwapInt32(&l.released, 0, 1) {
		return
	}
	if l.redis {
		releaseRedisChannelPoolSlot(l.key)
		return
	}
	releaseMemoryChannelPoolSlot(l.channelID)
}

func ChannelPoolConcurrencyLimit(channel *Channel) int {
	settings := parseChannelPoolOtherSettings(channel)
	if settings.ChannelPoolConcurrencyLimit < 0 {
		return 0
	}
	return settings.ChannelPoolConcurrencyLimit
}

func ChannelPoolCooldownSeconds(channel *Channel) int {
	settings := parseChannelPoolOtherSettings(channel)
	if settings.ChannelPoolCooldownSeconds < 0 {
		return 0
	}
	return settings.ChannelPoolCooldownSeconds
}

func ChannelPoolCandidateAvailable(channel *Channel, group string, modelName string) bool {
	return ChannelPoolCandidateStatusFor(channel, group, modelName).Available
}

func ChannelPoolCandidateStatusFor(channel *Channel, group string, modelName string) ChannelPoolCandidateStatus {
	status := ChannelPoolCandidateStatus{
		Reason: ChannelPoolCandidateReasonNoChannel,
	}
	if channel == nil {
		return status
	}
	status.ChannelID = channel.Id
	if isChannelPoolCoolingDown(channel.Id, group, modelName) {
		status.Reason = ChannelPoolCandidateReasonCooldown
		status.CoolingDown = true
		return status
	}
	limit := ChannelPoolConcurrencyLimit(channel)
	status.Limit = limit
	if limit <= 0 {
		status.Reason = ChannelPoolCandidateReasonAvailable
		status.Available = true
		return status
	}
	status.HasHardLimit = true
	status.Inflight = getChannelPoolInflight(channel.Id)
	if status.Inflight >= limit {
		status.Reason = ChannelPoolCandidateReasonFull
		return status
	}
	status.Reason = ChannelPoolCandidateReasonAvailable
	status.Available = true
	return status
}

func ChannelPoolSelectionSnapshotFor(group string, modelName string, requestPath string, options ChannelSelectionOptions) ChannelPoolSelectionSnapshot {
	snapshot := ChannelPoolSelectionSnapshot{
		CacheEnabled: common.MemoryCacheEnabled,
	}
	if !common.MemoryCacheEnabled {
		return snapshot
	}

	var candidates []*Channel
	channelSyncLock.RLock()
	if group2model2channels == nil {
		channelSyncLock.RUnlock()
		return snapshot
	}

	seen := make(map[int]struct{})
	collectModelCandidates := func(lookupModel string) {
		for _, channelID := range group2model2channels[group][lookupModel] {
			if _, ok := seen[channelID]; ok {
				continue
			}
			seen[channelID] = struct{}{}

			channel, ok := channelsIDM[channelID]
			if ok && !channelPoolPathAllowed(channelID, channel, requestPath) {
				snapshot.PathSkippedCount++
				continue
			}
			if _, skip := options.SkipChannelIDs[channelID]; skip {
				snapshot.SelectionSkippedCount++
				continue
			}
			if !ok {
				snapshot.MissingChannelCount++
				continue
			}

			snapshot.CandidateCount++
			candidates = append(candidates, channel)
		}
	}

	collectModelCandidates(modelName)
	normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
	if normalizedModel != modelName {
		collectModelCandidates(normalizedModel)
	}
	channelSyncLock.RUnlock()

	for _, channel := range candidates {
		status := ChannelPoolCandidateStatusFor(channel, group, modelName)
		switch status.Reason {
		case ChannelPoolCandidateReasonFull:
			snapshot.FullCount++
		case ChannelPoolCandidateReasonCooldown:
			snapshot.CooldownCount++
		default:
			if status.Available {
				snapshot.AvailableCount++
			}
		}
	}
	return snapshot
}

func AcquireChannelPoolLease(channel *Channel) (*ChannelPoolLease, bool, error) {
	if channel == nil {
		return nil, false, fmt.Errorf("channel is nil")
	}
	limit := ChannelPoolConcurrencyLimit(channel)
	if limit <= 0 {
		return nil, true, nil
	}
	if channelPoolRedisAvailable() {
		return acquireRedisChannelPoolSlot(channel.Id, limit)
	}
	lease := acquireMemoryChannelPoolSlot(channel.Id, limit)
	return lease, lease != nil, nil
}

func channelPoolPathAllowed(channelID int, channel *Channel, requestPath string) bool {
	if requestPath == "" || channel == nil || channel.Type != constant.ChannelTypeAdvancedCustom {
		return true
	}
	config := channel2advancedCustomConfig[channelID]
	return config != nil && config.SupportsPath(requestPath)
}

func CooldownChannelPool(channelID int, group string, modelName string, seconds int, reason string) {
	if channelID <= 0 || seconds <= 0 {
		return
	}
	key := channelPoolCooldownKey(channelID, group, modelName)
	expiration := time.Duration(seconds) * time.Second
	if channelPoolRedisAvailable() {
		if err := common.RDB.Set(context.Background(), key, reason, expiration).Err(); err != nil {
			common.SysError(fmt.Sprintf("channel pool cooldown set failed: channel_id=%d, err=%v", channelID, err))
		}
		return
	}
	channelPoolMemoryMu.Lock()
	channelPoolMemoryCooldown[key] = time.Now().Add(expiration)
	channelPoolMemoryMu.Unlock()
}

func filterChannelsBySelectionOptions(channels []int, options ChannelSelectionOptions) []int {
	if len(channels) == 0 || len(options.SkipChannelIDs) == 0 {
		return channels
	}
	filtered := make([]int, 0, len(channels))
	for _, channelID := range channels {
		if _, skip := options.SkipChannelIDs[channelID]; skip {
			continue
		}
		filtered = append(filtered, channelID)
	}
	return filtered
}

func filterChannelsByChannelPoolAvailability(channels []int, group string, modelName string) []int {
	if len(channels) == 0 {
		return channels
	}
	filtered := make([]int, 0, len(channels))
	for _, channelID := range channels {
		channel, ok := channelsIDM[channelID]
		if !ok {
			filtered = append(filtered, channelID)
			continue
		}
		if ChannelPoolCandidateAvailable(channel, group, modelName) {
			filtered = append(filtered, channelID)
		}
	}
	return filtered
}

func filterAbilitiesBySelectionOptions(abilities []Ability, options ChannelSelectionOptions) []Ability {
	if len(abilities) == 0 || len(options.SkipChannelIDs) == 0 {
		return abilities
	}
	filtered := make([]Ability, 0, len(abilities))
	for _, ability := range abilities {
		if _, skip := options.SkipChannelIDs[ability.ChannelId]; skip {
			continue
		}
		filtered = append(filtered, ability)
	}
	return filtered
}

func filterAbilitiesByChannelPoolAvailability(abilities []Ability, group string, modelName string) []Ability {
	if len(abilities) == 0 {
		return abilities
	}

	channelIDs := make([]int, 0, len(abilities))
	seen := make(map[int]struct{}, len(abilities))
	for _, ability := range abilities {
		if _, ok := seen[ability.ChannelId]; ok {
			continue
		}
		seen[ability.ChannelId] = struct{}{}
		channelIDs = append(channelIDs, ability.ChannelId)
	}

	var channels []*Channel
	if err := DB.Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		common.SysError(fmt.Sprintf("channel pool availability query failed: err=%v", err))
		return abilities
	}
	channelByID := make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		channelByID[channel.Id] = channel
	}

	filtered := make([]Ability, 0, len(abilities))
	for _, ability := range abilities {
		channel, ok := channelByID[ability.ChannelId]
		if !ok || ChannelPoolCandidateAvailable(channel, group, modelName) {
			filtered = append(filtered, ability)
		}
	}
	return filtered
}

func parseChannelPoolOtherSettings(channel *Channel) dto.ChannelOtherSettings {
	settings := dto.ChannelOtherSettings{}
	if channel == nil || channel.OtherSettings == "" {
		return settings
	}
	if err := common.UnmarshalJsonStr(channel.OtherSettings, &settings); err != nil {
		common.SysError(fmt.Sprintf("failed to unmarshal channel pool settings: channel_id=%d, err=%v", channel.Id, err))
	}
	return settings
}

func channelPoolRedisAvailable() bool {
	return common.RedisEnabled && common.RDB != nil
}

func channelPoolCooldownKey(channelID int, group string, modelName string) string {
	return fmt.Sprintf("%s:%d:%s", channelPoolCooldownNamespace, channelID, channelPoolScopeFingerprint(group, modelName))
}

func channelPoolConcurrencyKey(channelID int) string {
	return fmt.Sprintf("%s:%d", channelPoolConcurrencyNamespace, channelID)
}

func channelPoolScopeFingerprint(group string, modelName string) string {
	sum := sha1.Sum([]byte(group + "\x00" + modelName))
	return hex.EncodeToString(sum[:])
}

func isChannelPoolCoolingDown(channelID int, group string, modelName string) bool {
	key := channelPoolCooldownKey(channelID, group, modelName)
	if channelPoolRedisAvailable() {
		count, err := common.RDB.Exists(context.Background(), key).Result()
		if err != nil {
			common.SysError(fmt.Sprintf("channel pool cooldown check failed: channel_id=%d, err=%v", channelID, err))
			return false
		}
		return count > 0
	}

	channelPoolMemoryMu.Lock()
	defer channelPoolMemoryMu.Unlock()
	until, ok := channelPoolMemoryCooldown[key]
	if !ok {
		return false
	}
	if time.Now().After(until) {
		delete(channelPoolMemoryCooldown, key)
		return false
	}
	return true
}

func getChannelPoolInflight(channelID int) int {
	if channelID <= 0 {
		return 0
	}
	if channelPoolRedisAvailable() {
		value, err := common.RDB.Get(context.Background(), channelPoolConcurrencyKey(channelID)).Result()
		if err != nil {
			return 0
		}
		inflight, err := strconv.Atoi(value)
		if err != nil {
			return 0
		}
		return inflight
	}

	channelPoolMemoryMu.Lock()
	defer channelPoolMemoryMu.Unlock()
	return channelPoolMemoryInflight[channelID]
}

func acquireRedisChannelPoolSlot(channelID int, limit int) (*ChannelPoolLease, bool, error) {
	key := channelPoolConcurrencyKey(channelID)
	result, err := common.RDB.Eval(context.Background(), `
local current = tonumber(redis.call("GET", KEYS[1]) or "0")
local limit = tonumber(ARGV[1])
if current >= limit then
  return 0
end
current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("EXPIRE", KEYS[1], tonumber(ARGV[2]))
end
if current > limit then
  redis.call("DECR", KEYS[1])
  return 0
end
return current
`, []string{key}, limit, int(channelPoolLeaseTTL.Seconds())).Int()
	if err != nil {
		return nil, false, err
	}
	if result <= 0 {
		return nil, false, nil
	}
	return &ChannelPoolLease{channelID: channelID, key: key, redis: true}, true, nil
}

func releaseRedisChannelPoolSlot(key string) {
	if key == "" || !channelPoolRedisAvailable() {
		return
	}
	if err := common.RDB.Eval(context.Background(), `
local current = tonumber(redis.call("GET", KEYS[1]) or "0")
if current <= 1 then
  redis.call("DEL", KEYS[1])
  return 0
end
return redis.call("DECR", KEYS[1])
`, []string{key}).Err(); err != nil {
		common.SysError(fmt.Sprintf("channel pool slot release failed: key=%s, err=%v", key, err))
	}
}

func acquireMemoryChannelPoolSlot(channelID int, limit int) *ChannelPoolLease {
	channelPoolMemoryMu.Lock()
	defer channelPoolMemoryMu.Unlock()
	if channelPoolMemoryInflight[channelID] >= limit {
		return nil
	}
	channelPoolMemoryInflight[channelID]++
	return &ChannelPoolLease{channelID: channelID}
}

func releaseMemoryChannelPoolSlot(channelID int) {
	channelPoolMemoryMu.Lock()
	defer channelPoolMemoryMu.Unlock()
	if channelPoolMemoryInflight[channelID] <= 1 {
		delete(channelPoolMemoryInflight, channelID)
		return
	}
	channelPoolMemoryInflight[channelID]--
}
