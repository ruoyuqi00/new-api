package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

var userConcurrencyLimiter = newInMemoryConcurrencyLimiter()

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
		if !userConcurrencyLimiter.tryAcquire(key, limit) {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("concurrent request limit reached: maximum %d in-flight requests", limit))
			return
		}
		defer userConcurrencyLimiter.release(key, limit)

		c.Next()
	}
}
