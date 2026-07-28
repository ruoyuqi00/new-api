package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

var userConcurrencyLimiter = newInMemoryConcurrencyLimiter()

const (
	userConcurrencyNamespace = "new-api:user:concurrency:v1"
	userConcurrencyLeaseTTL  = 6 * time.Hour
)

type userConcurrencyLease struct {
	release  func()
	released int32
}

func (l *userConcurrencyLease) Release() {
	if l == nil || !atomic.CompareAndSwapInt32(&l.released, 0, 1) {
		return
	}
	l.release()
}

type inMemoryConcurrencyLimiter struct {
	mu       sync.Mutex
	counters map[string]int
}

func newInMemoryConcurrencyLimiter() *inMemoryConcurrencyLimiter {
	return &inMemoryConcurrencyLimiter{counters: make(map[string]int)}
}

func (l *inMemoryConcurrencyLimiter) tryAcquire(key string, limit int) bool {
	if limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.counters[key] >= limit {
		return false
	}
	l.counters[key]++
	return true
}

func (l *inMemoryConcurrencyLimiter) release(key string, limit int) {
	if limit <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.counters[key] <= 1 {
		delete(l.counters, key)
		return
	}
	l.counters[key]--
}

func acquireUserConcurrencyLease(key string, limit int) (*userConcurrencyLease, bool) {
	if common.RedisEnabled && common.RDB != nil {
		redisKey := userConcurrencyNamespace + ":" + key
		result, err := common.RDB.Eval(context.Background(), `
local current = tonumber(redis.call("GET", KEYS[1]) or "0")
local limit = tonumber(ARGV[1])
if current >= limit then
  return 0
end
current = redis.call("INCR", KEYS[1])
redis.call("EXPIRE", KEYS[1], tonumber(ARGV[2]))
return current
`, []string{redisKey}, limit, int(userConcurrencyLeaseTTL.Seconds())).Int()
		if err == nil {
			if result <= 0 {
				return nil, false
			}
			return &userConcurrencyLease{release: func() {
				if err := common.RDB.Eval(context.Background(), `
local current = tonumber(redis.call("GET", KEYS[1]) or "0")
if current <= 1 then
  redis.call("DEL", KEYS[1])
  return 0
end
return redis.call("DECR", KEYS[1])
`, []string{redisKey}).Err(); err != nil {
					common.SysError(fmt.Sprintf("user concurrency release failed: user=%s, err=%v", key, err))
				}
			}}, true
		}
		common.SysError(fmt.Sprintf("user concurrency acquire failed, using process-local fallback: user=%s, err=%v", key, err))
	}

	if !userConcurrencyLimiter.tryAcquire(key, limit) {
		return nil, false
	}
	return &userConcurrencyLease{release: func() {
		userConcurrencyLimiter.release(key, limit)
	}}, true
}

// UserConcurrencyLimit caps in-flight model requests per authenticated user.
func UserConcurrencyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !setting.UserConcurrencyLimitEnabled {
			c.Next()
			return
		}

		userID := c.GetInt("id")
		if userID <= 0 {
			c.Next()
			return
		}

		limit := setting.UserConcurrencyLimit
		group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		if group == "" {
			group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		}
		if groupLimit, found := setting.GetGroupConcurrencyLimit(group); found {
			limit = groupLimit
		}

		if limit <= 0 {
			c.Next()
			return
		}

		key := strconv.Itoa(userID)
		lease, acquired := acquireUserConcurrencyLease(key, limit)
		if !acquired {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("concurrent request limit reached: maximum %d in-flight requests", limit))
			return
		}
		defer lease.Release()

		c.Next()
	}
}
