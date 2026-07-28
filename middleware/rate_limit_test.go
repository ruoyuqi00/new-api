package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func exerciseRateLimit(t *testing.T, limiter gin.HandlerFunc) {
	t.Helper()
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.GET("/limited", limiter, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "192.0.2.10:12345"
		router.ServeHTTP(recorder, req)
		return recorder
	}

	assert.Equal(t, http.StatusNoContent, request().Code)
	limited := request()
	assert.Equal(t, http.StatusTooManyRequests, limited.Code)
	assert.Equal(t, "37", limited.Header().Get("Retry-After"))
}

func TestMemoryRateLimiterAddsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRedisEnabled := common.RedisEnabled
	previousLimiter := inMemoryRateLimiter
	common.RedisEnabled = false
	inMemoryRateLimiter = common.InMemoryRateLimiter{}
	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
		inMemoryRateLimiter = previousLimiter
	})

	exerciseRateLimit(t, rateLimitFactory(1, 37, "TEST_MEMORY"))
}

func TestRedisRateLimiterAddsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := miniredis.RunT(t)
	previousRedisEnabled := common.RedisEnabled
	previousRDB := common.RDB
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRDB
	})

	exerciseRateLimit(t, rateLimitFactory(1, 37, "TEST_REDIS"))
}
