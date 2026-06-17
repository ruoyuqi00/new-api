package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const stickySessionPrefix = "sticky_session:"
const accountSelectionCooldownPrefix = "account_selection_cooldown:"

type gatewayCache struct {
	rdb *redis.Client
}

func NewGatewayCache(rdb *redis.Client) service.GatewayCache {
	return &gatewayCache{rdb: rdb}
}

// buildSessionKey 构建 session key，包含 groupID 实现分组隔离
// 格式: sticky_session:{groupID}:{sessionHash}
func buildSessionKey(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%s%d:%s", stickySessionPrefix, groupID, sessionHash)
}

func buildAccountSelectionCooldownKey(groupID int64, model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "_all"
	}
	sum := sha256.Sum256([]byte(model))
	return fmt.Sprintf("%s%d:%x", accountSelectionCooldownPrefix, groupID, sum[:8])
}

func (c *gatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Get(ctx, key).Int64()
}

func (c *gatewayCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Set(ctx, key, accountID, ttl).Err()
}

func (c *gatewayCache) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// DeleteSessionAccountID 删除粘性会话与账号的绑定关系。
// 当检测到绑定的账号不可用（如状态错误、禁用、不可调度等）时调用，
// 以便下次请求能够重新选择可用账号。
//
// DeleteSessionAccountID removes the sticky session binding for the given session.
// Called when the bound account becomes unavailable (e.g., error status, disabled,
// or unschedulable), allowing subsequent requests to select a new available account.
func (c *gatewayCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Del(ctx, key).Err()
}

func (c *gatewayCache) SetAccountSelectionCooldowns(ctx context.Context, groupID int64, model string, accountIDs []int64, ttl time.Duration) error {
	if len(accountIDs) == 0 || ttl <= 0 {
		return nil
	}
	key := buildAccountSelectionCooldownKey(groupID, model)
	now := time.Now().UTC()
	untilMs := now.Add(ttl).UnixMilli()
	pipe := c.rdb.Pipeline()
	for _, accountID := range accountIDs {
		if accountID <= 0 {
			continue
		}
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  float64(untilMs),
			Member: strconv.FormatInt(accountID, 10),
		})
	}
	pipe.Expire(ctx, key, ttl+time.Minute)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *gatewayCache) GetAccountSelectionCooldowns(ctx context.Context, groupID int64, model string, accountIDs []int64) (map[int64]struct{}, error) {
	if len(accountIDs) == 0 {
		return map[int64]struct{}{}, nil
	}
	key := buildAccountSelectionCooldownKey(groupID, model)
	nowMs := time.Now().UTC().UnixMilli()
	_, _ = c.rdb.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(nowMs, 10)).Result()

	values, err := c.rdb.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: strconv.FormatInt(nowMs+1, 10),
		Max: "+inf",
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return map[int64]struct{}{}, nil
		}
		return nil, err
	}

	if len(values) == 0 {
		return map[int64]struct{}{}, nil
	}

	lookup := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		if accountID > 0 {
			lookup[accountID] = struct{}{}
		}
	}

	out := make(map[int64]struct{})
	for _, value := range values {
		accountID, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil {
			continue
		}
		if _, ok := lookup[accountID]; ok {
			out[accountID] = struct{}{}
		}
	}
	return out, nil
}
