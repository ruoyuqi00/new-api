package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestGatewayCacheAccountSelectionCooldowns(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	cache := NewGatewayCache(rdb)
	cooldownCache, ok := cache.(interface {
		SetAccountSelectionCooldowns(context.Context, int64, string, []int64, time.Duration) error
		GetAccountSelectionCooldowns(context.Context, int64, string, []int64) (map[int64]struct{}, error)
	})
	require.True(t, ok)

	ctx := context.Background()
	require.NoError(t, cooldownCache.SetAccountSelectionCooldowns(ctx, 7, "claude-test", []int64{11, 22}, 20*time.Millisecond))

	hits, err := cooldownCache.GetAccountSelectionCooldowns(ctx, 7, "claude-test", []int64{11, 33})
	require.NoError(t, err)
	require.Contains(t, hits, int64(11))
	require.NotContains(t, hits, int64(22), "only requested candidate IDs should be returned")
	require.NotContains(t, hits, int64(33))

	time.Sleep(30 * time.Millisecond)
	hits, err = cooldownCache.GetAccountSelectionCooldowns(ctx, 7, "claude-test", []int64{11, 22})
	require.NoError(t, err)
	require.Empty(t, hits)
}
