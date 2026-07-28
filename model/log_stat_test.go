package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSumUsedQuotaFiltersRequestIds(t *testing.T) {
	truncateTables(t)

	now := common.GetTimestamp()
	logs := []Log{
		{
			UserId:            1,
			Username:          "alice",
			CreatedAt:         now,
			Type:              LogTypeConsume,
			ModelName:         "gpt-a",
			TokenName:         "tok-a",
			Quota:             100,
			PromptTokens:      10,
			CompletionTokens:  5,
			ChannelId:         7,
			Group:             "vip",
			RequestId:         "req-a",
			UpstreamRequestId: "up-a",
		},
		{
			UserId:            1,
			Username:          "alice",
			CreatedAt:         now,
			Type:              LogTypeConsume,
			ModelName:         "gpt-a",
			TokenName:         "tok-a",
			Quota:             200,
			PromptTokens:      20,
			CompletionTokens:  10,
			ChannelId:         7,
			Group:             "vip",
			RequestId:         "req-b",
			UpstreamRequestId: "up-a",
		},
		{
			UserId:            1,
			Username:          "alice",
			CreatedAt:         now,
			Type:              LogTypeConsume,
			ModelName:         "gpt-a",
			TokenName:         "tok-a",
			Quota:             300,
			PromptTokens:      30,
			CompletionTokens:  15,
			ChannelId:         7,
			Group:             "vip",
			RequestId:         "req-a",
			UpstreamRequestId: "up-b",
		},
		{
			UserId:            1,
			Username:          "alice",
			CreatedAt:         now,
			Type:              LogTypeError,
			ModelName:         "gpt-a",
			TokenName:         "tok-a",
			Quota:             999,
			PromptTokens:      99,
			CompletionTokens:  99,
			ChannelId:         7,
			Group:             "vip",
			RequestId:         "req-a",
			UpstreamRequestId: "up-a",
		},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	stat, err := SumUsedQuota(LogTypeUnknown, now-1, now+1, "gpt-a", "alice", "tok-a", 7, "vip", "req-a", "")
	require.NoError(t, err)
	assert.Equal(t, 400, stat.Quota)
	assert.Equal(t, 2, stat.Rpm)
	assert.Equal(t, 60, stat.Tpm)

	stat, err = SumUsedQuota(LogTypeUnknown, now-1, now+1, "gpt-a", "alice", "tok-a", 7, "vip", "", "up-a")
	require.NoError(t, err)
	assert.Equal(t, 300, stat.Quota)
	assert.Equal(t, 2, stat.Rpm)
	assert.Equal(t, 45, stat.Tpm)

	stat, err = SumUsedQuota(LogTypeUnknown, now-1, now+1, "gpt-a", "alice", "tok-a", 7, "vip", "req-a", "up-a")
	require.NoError(t, err)
	assert.Equal(t, 100, stat.Quota)
	assert.Equal(t, 1, stat.Rpm)
	assert.Equal(t, 15, stat.Tpm)

	stat, err = SumUsedQuota(LogTypeUnknown, now-1, now+1, "gpt-a", "alice", "tok-a", 7, "vip", "req-missing", "")
	require.NoError(t, err)
	assert.Equal(t, 0, stat.Quota)
	assert.Equal(t, 0, stat.Rpm)
	assert.Equal(t, 0, stat.Tpm)
}

func TestSumUsedQuotaAlwaysReportsConsumeStats(t *testing.T) {
	truncateTables(t)

	now := common.GetTimestamp()
	logs := []Log{
		{
			UserId:           2,
			Username:         "bob",
			CreatedAt:        now,
			Type:             LogTypeConsume,
			ModelName:        "gpt-b",
			TokenName:        "tok-b",
			Quota:            100,
			PromptTokens:     10,
			CompletionTokens: 5,
			ChannelId:        8,
			Group:            "vip",
		},
		{
			UserId:           2,
			Username:         "bob",
			CreatedAt:        now,
			Type:             LogTypeRefund,
			ModelName:        "gpt-b",
			TokenName:        "tok-b",
			Quota:            900,
			PromptTokens:     90,
			CompletionTokens: 90,
			ChannelId:        8,
			Group:            "vip",
		},
		{
			UserId:           2,
			Username:         "bob",
			CreatedAt:        now,
			Type:             LogTypeTopup,
			ModelName:        "gpt-b",
			TokenName:        "tok-b",
			Quota:            700,
			PromptTokens:     70,
			CompletionTokens: 70,
			ChannelId:        8,
			Group:            "vip",
		},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	stat, err := SumUsedQuota(LogTypeRefund, now-1, now+1, "gpt-b", "bob", "tok-b", 8, "vip", "", "")
	require.NoError(t, err)
	assert.Equal(t, 100, stat.Quota)
	assert.Equal(t, 1, stat.Rpm)
	assert.Equal(t, 15, stat.Tpm)

	stat, err = SumUsedQuota(LogTypeTopup, now-1, now+1, "gpt-b", "bob", "tok-b", 8, "vip", "", "")
	require.NoError(t, err)
	assert.Equal(t, 100, stat.Quota)
	assert.Equal(t, 1, stat.Rpm)
	assert.Equal(t, 15, stat.Tpm)
}
