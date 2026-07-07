package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

func TestUserConcurrencyLimitRejectsOverflowAndReleases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldEnabled := setting.UserConcurrencyLimitEnabled
	oldLimit := setting.UserConcurrencyLimit
	oldGroup := setting.UserConcurrencyLimitGroup
	defer func() {
		setting.UserConcurrencyLimitEnabled = oldEnabled
		setting.UserConcurrencyLimit = oldLimit
		setting.UserConcurrencyLimitGroup = oldGroup
		userConcurrencyLimiter = newInMemoryConcurrencyLimiter()
	}()

	setting.UserConcurrencyLimitEnabled = true
	setting.UserConcurrencyLimit = 1
	setting.UserConcurrencyLimitGroup = map[string]int{}
	userConcurrencyLimiter = newInMemoryConcurrencyLimiter()

	block := make(chan struct{})
	started := make(chan struct{})
	var startedOnce sync.Once
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 42)
		c.Next()
	})
	router.Use(UserConcurrencyLimit())
	router.GET("/relay", func(c *gin.Context) {
		startedOnce.Do(func() { close(started) })
		<-block
		c.Status(http.StatusOK)
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/relay", nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("first request status = %d, want %d", rec.Code, http.StatusOK)
		}
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first request did not start")
	}

	overflow := httptest.NewRecorder()
	router.ServeHTTP(overflow, httptest.NewRequest(http.MethodGet, "/relay", nil))
	if overflow.Code != http.StatusTooManyRequests {
		t.Fatalf("overflow status = %d, want %d", overflow.Code, http.StatusTooManyRequests)
	}

	close(block)
	wg.Wait()

	released := httptest.NewRecorder()
	router.ServeHTTP(released, httptest.NewRequest(http.MethodGet, "/relay", nil))
	if released.Code != http.StatusOK {
		t.Fatalf("released status = %d, want %d", released.Code, http.StatusOK)
	}
}
