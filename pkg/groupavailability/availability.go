package groupavailability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const (
	maxSamples              = 300
	sampleTTL               = 2 * time.Hour
	AvailabilityStable      = "stable"
	AvailabilityDegraded    = "degraded"
	AvailabilityUnavailable = "unavailable"
	AvailabilityNoData      = "no_data"
)

type Summary struct {
	Group        string
	RequestCount int64
	SuccessCount int64
	SuccessRate  float64
	Status       string
	ObservedAt   int64
}

func Record(group string, success bool) error {
	group = strings.TrimSpace(group)
	if group == "" || !ratio_setting.IsAvailabilityMonitoringEnabled(group) {
		return nil
	}
	if !common.RedisEnabled || common.RDB == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	value := "0|" + strconv.FormatInt(time.Now().Unix(), 10)
	if success {
		value = "1|" + strconv.FormatInt(time.Now().Unix(), 10)
	}
	key := redisKey(group)
	pipe := common.RDB.TxPipeline()
	pipe.LPush(ctx, key, value)
	pipe.LTrim(ctx, key, 0, maxSamples-1)
	pipe.Expire(ctx, key, sampleTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func Query(group string) (Summary, error) {
	group = strings.TrimSpace(group)
	summary := Summary{Group: group, Status: AvailabilityNoData}
	if group == "" || !common.RedisEnabled || common.RDB == nil {
		return summary, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	values, err := common.RDB.LRange(ctx, redisKey(group), 0, maxSamples-1).Result()
	if err != nil {
		return summary, nil
	}
	for _, value := range values {
		parts := strings.SplitN(value, "|", 2)
		if len(parts) != 2 || (parts[0] != "0" && parts[0] != "1") {
			continue
		}
		summary.RequestCount++
		if parts[0] == "1" {
			summary.SuccessCount++
		}
		if observedAt, parseErr := strconv.ParseInt(parts[1], 10, 64); parseErr == nil && observedAt > summary.ObservedAt {
			summary.ObservedAt = observedAt
		}
	}
	if summary.RequestCount == 0 {
		return summary, nil
	}
	summary.SuccessRate = float64(summary.SuccessCount) / float64(summary.RequestCount) * 100
	summary.SuccessRate = float64(int(summary.SuccessRate*100+0.5)) / 100
	summary.Status = statusFor(summary.SuccessRate)
	return summary, nil
}

func statusFor(successRate float64) string {
	if successRate >= 99 {
		return AvailabilityStable
	}
	if successRate >= 95 {
		return AvailabilityDegraded
	}
	return AvailabilityUnavailable
}

func redisKey(group string) string {
	digest := sha256.Sum256([]byte(group))
	return fmt.Sprintf("group-availability:v1:%s", hex.EncodeToString(digest[:]))
}

func IsTextRequestPath(path string) bool {
	path = strings.TrimSuffix(strings.SplitN(path, "?", 2)[0], "/")
	switch path {
	case "/v1/chat/completions", "/v1/responses", "/v1/responses/compact", "/v1/completions":
		return true
	default:
		return false
	}
}
