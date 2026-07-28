package model

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	providerAccountConcurrencyNamespace = "new-api:provider_account:concurrency:v1"
	providerAccountCooldownNamespace    = "new-api:provider_account:cooldown:v1"
	providerAccountLeaseTTL             = 6 * time.Hour
)

var (
	providerAccountRuntimeMu      sync.Mutex
	providerAccountMemoryInflight = map[int]int{}
	providerAccountMemoryCooldown = map[int]time.Time{}
	accountPoolCacheMu            sync.RWMutex
	accountPoolsByChannel         = map[int][]accountPoolRuntimeEntry{}
	accountPoolsByAdapter         = map[int][]accountPoolRuntimeEntry{}
	providerAccountHealthByID     = map[int]providerAccountRuntimeHealth{}
)

type accountPoolRuntimeEntry struct {
	Pool     AccountPool
	Accounts []ProviderAccount
}

type providerAccountRuntimeHealth struct {
	blocked          bool
	unavailableUntil int64
}

type ProviderAccountLease struct {
	AccountId       int
	PoolId          int
	AccountName     string
	Credential      string
	BaseURL         string
	ModelMapping    string
	CooldownSeconds int
	tracked         bool
	redis           bool
	key             string
	released        int32
}

type ProviderAccountSelectionOptions struct {
	SkipAccountIDs                   map[int]struct{}
	RequireGrokMediaGenerationAccess bool
}

func (l *ProviderAccountLease) Release() {
	if l == nil || !atomic.CompareAndSwapInt32(&l.released, 0, 1) {
		return
	}
	if !l.tracked {
		return
	}
	if l.redis {
		releaseRedisProviderAccountSlot(l.key)
		return
	}
	providerAccountRuntimeMu.Lock()
	if providerAccountMemoryInflight[l.AccountId] <= 1 {
		delete(providerAccountMemoryInflight, l.AccountId)
	} else {
		providerAccountMemoryInflight[l.AccountId]--
	}
	providerAccountRuntimeMu.Unlock()
}

func AcquireProviderAccount(channelId int, adapterType int, group string) (*ProviderAccountLease, bool, error) {
	return AcquireProviderAccountWithOptions(channelId, adapterType, group, ProviderAccountSelectionOptions{})
}

func AcquireProviderAccountWithOptions(channelId int, adapterType int, group string, options ProviderAccountSelectionOptions) (*ProviderAccountLease, bool, error) {
	var pools []AccountPool
	accountsByPool := make(map[int][]ProviderAccount)
	healthByAccount := make(map[int]providerAccountRuntimeHealth)
	if common.MemoryCacheEnabled {
		accountPoolCacheMu.RLock()
		entries, bound := accountPoolsByChannel[channelId]
		entries = append(entries, accountPoolsByAdapter[adapterType]...)
		bound = bound || len(entries) > 0
		for _, entry := range entries {
			pools = append(pools, entry.Pool)
			accountsByPool[entry.Pool.Id] = append([]ProviderAccount(nil), entry.Accounts...)
		}
		healthByAccount = providerAccountHealthByID
		accountPoolCacheMu.RUnlock()
		if !bound {
			return nil, false, nil
		}
	} else {
		if err := DB.Table("account_pools").
			Where("EXISTS (SELECT 1 FROM channel_account_pool_bindings WHERE channel_account_pool_bindings.pool_id = account_pools.id AND channel_account_pool_bindings.channel_id = ? AND channel_account_pool_bindings.enabled = ?) OR (account_pools.adapter_type = ? AND NOT EXISTS (SELECT 1 FROM channel_account_pool_bindings WHERE channel_account_pool_bindings.pool_id = account_pools.id AND channel_account_pool_bindings.enabled = ?))", channelId, true, adapterType, true).
			Order("account_pools.priority DESC, account_pools.id ASC").
			Find(&pools).Error; err != nil {
			return nil, true, err
		}
	}
	sort.Slice(pools, func(i, j int) bool {
		if pools[i].Priority == pools[j].Priority {
			return pools[i].Id < pools[j].Id
		}
		return pools[i].Priority > pools[j].Priority
	})
	availablePools := make([]AccountPool, 0, len(pools))
	for _, pool := range pools {
		if pool.Status == AccountPoolStatusEnabled && accountPoolAllowsGroup(pool.Group, group) {
			availablePools = append(availablePools, pool)
		}
	}
	if len(availablePools) == 0 {
		return nil, false, nil
	}

	skippedAccounts := make(map[int]struct{})
	skippedPools := make(map[int]struct{})
	coolingAccountsByPool := make(map[int]map[int]struct{})
	for attempts := 0; attempts < 64; attempts++ {
		activePools := make([]AccountPool, 0, len(availablePools))
		activePoolWeights := make([]int, 0, len(availablePools))
		var activePriority int64
		prioritySet := false
		for _, candidate := range availablePools {
			if _, skipped := skippedPools[candidate.Id]; skipped {
				continue
			}
			if !prioritySet {
				activePriority = candidate.Priority
				prioritySet = true
			}
			if candidate.Priority != activePriority {
				break
			}
			activePools = append(activePools, candidate)
			activePoolWeights = append(activePoolWeights, int(candidate.Weight))
		}
		poolIndex := selectChannelIndexByWeight(activePoolWeights, common.GetRandomInt)
		if poolIndex < 0 {
			break
		}
		pool := activePools[poolIndex]
		accounts := accountsByPool[pool.Id]
		now := common.GetTimestamp()
		if !common.MemoryCacheEnabled {
			if err := DB.Where("pool_id = ?", pool.Id).Order("priority DESC, id ASC").Find(&accounts).Error; err != nil {
				return nil, true, err
			}
		}
		coolingAccounts, checkedCooling := coolingAccountsByPool[pool.Id]
		if !checkedCooling {
			coolingAccounts = providerAccountsCoolingDown(accounts)
			coolingAccountsByPool[pool.Id] = coolingAccounts
		}
		availableAccounts := make([]ProviderAccount, 0, len(accounts))
		for _, account := range accounts {
			if _, skipped := options.SkipAccountIDs[account.Id]; skipped {
				continue
			}
			if account.Status != ProviderAccountEnabled || (account.ExpiresAt > 0 && account.ExpiresAt <= now) {
				continue
			}
			if options.RequireGrokMediaGenerationAccess && !providerAccountAllowsGrokMediaGeneration(account) {
				continue
			}
			health, healthKnown := healthByAccount[account.Id]
			if !common.MemoryCacheEnabled {
				health, healthKnown = providerAccountRoutingHealth(account, now)
			}
			if healthKnown && health.blocked && (health.unavailableUntil == 0 || health.unavailableUntil > now) {
				continue
			}
			if _, skipped := skippedAccounts[account.Id]; skipped {
				continue
			}
			if _, coolingDown := coolingAccounts[account.Id]; coolingDown {
				continue
			}
			availableAccounts = append(availableAccounts, account)
		}
		if len(availableAccounts) == 0 {
			skippedPools[pool.Id] = struct{}{}
			continue
		}
		highestAccountPriority := availableAccounts[0].Priority
		accountCandidates := make([]ProviderAccount, 0, len(availableAccounts))
		accountWeights := make([]int, 0, len(availableAccounts))
		for _, account := range availableAccounts {
			if account.Priority != highestAccountPriority {
				break
			}
			accountCandidates = append(accountCandidates, account)
			accountWeights = append(accountWeights, int(account.Weight))
		}
		accountIndex := selectChannelIndexByWeight(accountWeights, common.GetRandomInt)
		if accountIndex < 0 {
			skippedPools[pool.Id] = struct{}{}
			continue
		}
		account := accountCandidates[accountIndex]
		lease, acquired, err := acquireProviderAccountSlot(account)
		if err != nil {
			common.SysError(fmt.Sprintf("provider account slot acquire failed open: account_id=%d err=%v", account.Id, err))
			lease = &ProviderAccountLease{AccountId: account.Id, PoolId: account.PoolId, AccountName: account.Name, Credential: account.Credential, BaseURL: account.BaseURL, ModelMapping: account.ModelMapping, CooldownSeconds: account.CooldownSeconds}
			acquired = true
		}
		if acquired {
			return lease, true, nil
		}
		skippedAccounts[account.Id] = struct{}{}
	}
	return nil, true, fmt.Errorf("all provider accounts are unavailable for channel #%d", channelId)
}

func providerAccountAllowsGrokMediaGeneration(account ProviderAccount) bool {
	if !strings.EqualFold(strings.TrimSpace(account.Type), "oauth_json") {
		return true
	}

	var metadata struct {
		GrokMediaEligible *bool `json:"grok_media_eligible"`
	}
	if common.UnmarshalJsonStr(strings.TrimSpace(account.Metadata), &metadata) == nil && metadata.GrokMediaEligible != nil {
		return *metadata.GrokMediaEligible
	}

	var snapshot struct {
		Billing *struct {
			StatusCode        int `json:"status_code"`
			WeeklyStatusCode  int `json:"weekly_status_code"`
			MonthlyStatusCode int `json:"monthly_status_code"`
		} `json:"billing"`
	}
	if common.UnmarshalJsonStr(strings.TrimSpace(account.UsageSnapshot), &snapshot) != nil || snapshot.Billing == nil {
		return true
	}
	return snapshot.Billing.StatusCode != http.StatusForbidden &&
		snapshot.Billing.WeeklyStatusCode != http.StatusForbidden &&
		snapshot.Billing.MonthlyStatusCode != http.StatusForbidden
}

func InitAccountPoolCache() {
	if !common.MemoryCacheEnabled {
		return
	}
	var pools []AccountPool
	var accounts []ProviderAccount
	var bindings []ChannelAccountPoolBinding
	if err := DB.Find(&pools).Error; err != nil {
		common.SysError(fmt.Sprintf("account pool cache refresh failed: %v", err))
		return
	}
	if err := DB.Order("priority DESC, id ASC").Find(&accounts).Error; err != nil {
		common.SysError(fmt.Sprintf("provider account cache refresh failed: %v", err))
		return
	}
	if err := DB.Where("enabled = ?", true).Find(&bindings).Error; err != nil {
		common.SysError(fmt.Sprintf("account pool binding cache refresh failed: %v", err))
		return
	}
	poolsById := make(map[int]AccountPool, len(pools))
	accountsByPool := make(map[int][]ProviderAccount)
	nextHealthByAccount := make(map[int]providerAccountRuntimeHealth)
	now := common.GetTimestamp()
	for _, pool := range pools {
		poolsById[pool.Id] = pool
	}
	for _, account := range accounts {
		accountsByPool[account.PoolId] = append(accountsByPool[account.PoolId], account)
		if health, unavailable := providerAccountRoutingHealth(account, now); unavailable {
			nextHealthByAccount[account.Id] = health
		}
	}
	next := make(map[int][]accountPoolRuntimeEntry)
	nextByAdapter := make(map[int][]accountPoolRuntimeEntry)
	boundPoolIds := make(map[int]struct{})
	for _, binding := range bindings {
		pool, ok := poolsById[binding.PoolId]
		if !ok {
			continue
		}
		next[binding.ChannelId] = append(next[binding.ChannelId], accountPoolRuntimeEntry{Pool: pool, Accounts: accountsByPool[pool.Id]})
		boundPoolIds[pool.Id] = struct{}{}
	}
	for _, pool := range pools {
		if pool.AdapterType <= 0 {
			continue
		}
		if _, bound := boundPoolIds[pool.Id]; bound {
			continue
		}
		nextByAdapter[pool.AdapterType] = append(nextByAdapter[pool.AdapterType], accountPoolRuntimeEntry{Pool: pool, Accounts: accountsByPool[pool.Id]})
	}
	for channelId := range next {
		sort.Slice(next[channelId], func(i, j int) bool {
			if next[channelId][i].Pool.Priority == next[channelId][j].Pool.Priority {
				return next[channelId][i].Pool.Id < next[channelId][j].Pool.Id
			}
			return next[channelId][i].Pool.Priority > next[channelId][j].Pool.Priority
		})
	}
	for adapterType := range nextByAdapter {
		sort.Slice(nextByAdapter[adapterType], func(i, j int) bool {
			if nextByAdapter[adapterType][i].Pool.Priority == nextByAdapter[adapterType][j].Pool.Priority {
				return nextByAdapter[adapterType][i].Pool.Id < nextByAdapter[adapterType][j].Pool.Id
			}
			return nextByAdapter[adapterType][i].Pool.Priority > nextByAdapter[adapterType][j].Pool.Priority
		})
	}
	accountPoolCacheMu.Lock()
	accountPoolsByChannel = next
	accountPoolsByAdapter = nextByAdapter
	providerAccountHealthByID = nextHealthByAccount
	accountPoolCacheMu.Unlock()
}

func CooldownProviderAccount(accountId int, seconds int, reason string) {
	if accountId <= 0 || seconds <= 0 {
		return
	}
	if common.RedisEnabled && common.RDB != nil {
		key := providerAccountCooldownKey(accountId)
		if err := common.RDB.Set(context.Background(), key, reason, time.Duration(seconds)*time.Second).Err(); err != nil {
			common.SysError(fmt.Sprintf("provider account cooldown failed: account_id=%d err=%v", accountId, err))
		}
		return
	}
	providerAccountRuntimeMu.Lock()
	providerAccountMemoryCooldown[accountId] = time.Now().Add(time.Duration(seconds) * time.Second)
	providerAccountRuntimeMu.Unlock()
}

func accountPoolAllowsGroup(configured string, group string) bool {
	group = strings.TrimSpace(group)
	if group == "" {
		group = "default"
	}
	for _, candidate := range strings.Split(configured, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == group {
			return true
		}
	}
	return false
}

func providerAccountsCoolingDown(accounts []ProviderAccount) map[int]struct{} {
	coolingDown := make(map[int]struct{})
	if len(accounts) == 0 {
		return coolingDown
	}
	if common.RedisEnabled && common.RDB != nil {
		keys := make([]string, 0, len(accounts))
		accountIds := make([]int, 0, len(accounts))
		for _, account := range accounts {
			keys = append(keys, providerAccountCooldownKey(account.Id))
			accountIds = append(accountIds, account.Id)
		}
		values, err := common.RDB.MGet(context.Background(), keys...).Result()
		if err != nil {
			common.SysError(fmt.Sprintf("provider account cooldown batch check failed: %v", err))
			return coolingDown
		}
		for index, value := range values {
			if value != nil {
				coolingDown[accountIds[index]] = struct{}{}
			}
		}
		return coolingDown
	}
	providerAccountRuntimeMu.Lock()
	defer providerAccountRuntimeMu.Unlock()
	now := time.Now()
	for _, account := range accounts {
		until, ok := providerAccountMemoryCooldown[account.Id]
		if !ok {
			continue
		}
		if now.After(until) {
			delete(providerAccountMemoryCooldown, account.Id)
			continue
		}
		coolingDown[account.Id] = struct{}{}
	}
	return coolingDown
}

func providerAccountRoutingHealth(account ProviderAccount, now int64) (providerAccountRuntimeHealth, bool) {
	if account.UsageCheckedAt <= 0 {
		return providerAccountRuntimeHealth{}, false
	}
	switch account.UsageErrorCode {
	case "invalid_credential", "oauth_refresh_failed":
		return providerAccountRuntimeHealth{blocked: true}, true
	}
	switch account.UsageUpstreamStatus {
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden:
		return providerAccountRuntimeHealth{blocked: true}, true
	case http.StatusTooManyRequests:
		until := account.UsageCheckedAt + int64(max(account.CooldownSeconds, 300))
		if until > now {
			return providerAccountRuntimeHealth{blocked: true, unavailableUntil: until}, true
		}
		return providerAccountRuntimeHealth{}, false
	}
	if account.UsageUpstreamStatus < http.StatusOK || account.UsageUpstreamStatus >= http.StatusMultipleChoices || strings.TrimSpace(account.UsageSnapshot) == "" {
		return providerAccountRuntimeHealth{}, false
	}
	var snapshot struct {
		RateLimit *struct {
			Allowed      bool `json:"allowed"`
			LimitReached bool `json:"limit_reached"`
			Primary      *struct {
				ResetAt           int64   `json:"reset_at"`
				ResetAfterSeconds int64   `json:"reset_after_seconds"`
				UsedPercent       float64 `json:"used_percent"`
			} `json:"primary_window"`
			Secondary *struct {
				ResetAt           int64   `json:"reset_at"`
				ResetAfterSeconds int64   `json:"reset_after_seconds"`
				UsedPercent       float64 `json:"used_percent"`
			} `json:"secondary_window"`
		} `json:"rate_limit"`
	}
	if err := common.UnmarshalJsonStr(account.UsageSnapshot, &snapshot); err != nil || snapshot.RateLimit == nil || snapshot.RateLimit.Allowed {
		return providerAccountRuntimeHealth{}, false
	}
	until := int64(0)
	if window := snapshot.RateLimit.Primary; window != nil && (snapshot.RateLimit.LimitReached || window.UsedPercent >= 100) {
		until = window.ResetAt
		if until == 0 && window.ResetAfterSeconds > 0 {
			until = account.UsageCheckedAt + window.ResetAfterSeconds
		}
	}
	if window := snapshot.RateLimit.Secondary; window != nil && window.UsedPercent >= 100 {
		secondaryUntil := window.ResetAt
		if secondaryUntil == 0 && window.ResetAfterSeconds > 0 {
			secondaryUntil = account.UsageCheckedAt + window.ResetAfterSeconds
		}
		if secondaryUntil > until {
			until = secondaryUntil
		}
	}
	if until == 0 {
		until = account.UsageCheckedAt + 300
	}
	if until <= now {
		return providerAccountRuntimeHealth{}, false
	}
	return providerAccountRuntimeHealth{blocked: true, unavailableUntil: until}, true
}

func acquireProviderAccountSlot(account ProviderAccount) (*ProviderAccountLease, bool, error) {
	base := &ProviderAccountLease{AccountId: account.Id, PoolId: account.PoolId, AccountName: account.Name, Credential: account.Credential, BaseURL: account.BaseURL, ModelMapping: account.ModelMapping, CooldownSeconds: account.CooldownSeconds}
	if account.ConcurrencyLimit <= 0 {
		return base, true, nil
	}
	if common.RedisEnabled && common.RDB != nil {
		key := providerAccountConcurrencyKey(account.Id)
		result, err := common.RDB.Eval(context.Background(), `
local current = tonumber(redis.call("GET", KEYS[1]) or "0")
local limit = tonumber(ARGV[1])
if current >= limit then return 0 end
current = redis.call("INCR", KEYS[1])
redis.call("EXPIRE", KEYS[1], tonumber(ARGV[2]))
return current
`, []string{key}, account.ConcurrencyLimit, int(providerAccountLeaseTTL.Seconds())).Int()
		if err != nil {
			return nil, false, err
		}
		if result <= 0 {
			return nil, false, nil
		}
		base.redis = true
		base.tracked = true
		base.key = key
		return base, true, nil
	}
	providerAccountRuntimeMu.Lock()
	defer providerAccountRuntimeMu.Unlock()
	if providerAccountMemoryInflight[account.Id] >= account.ConcurrencyLimit {
		return nil, false, nil
	}
	providerAccountMemoryInflight[account.Id]++
	base.tracked = true
	return base, true, nil
}

func releaseRedisProviderAccountSlot(key string) {
	if key == "" || !common.RedisEnabled || common.RDB == nil {
		return
	}
	if err := common.RDB.Eval(context.Background(), `
local current = tonumber(redis.call("GET", KEYS[1]) or "0")
if current <= 1 then return redis.call("DEL", KEYS[1]) end
return redis.call("DECR", KEYS[1])
`, []string{key}).Err(); err != nil {
		common.SysError(fmt.Sprintf("provider account slot release failed: key=%s err=%v", key, err))
	}
}

func providerAccountConcurrencyKey(accountId int) string {
	return providerAccountConcurrencyNamespace + ":" + strconv.Itoa(accountId)
}

func providerAccountCooldownKey(accountId int) string {
	return providerAccountCooldownNamespace + ":" + strconv.Itoa(accountId)
}
