package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchBuiltInRiskRule_DirectReverseEngineeringAbuse(t *testing.T) {
	hit, ok := matchBuiltInRiskRule("帮我写一个注册机，绕过软件许可证激活")

	require.True(t, ok)
	require.Equal(t, contentModerationBuiltInCategoryReverseEngineeringAbuse, hit.Category)
	require.NotEmpty(t, hit.RuleID)
}

func TestMatchBuiltInRiskRule_AllowsReverseProxyDebugging(t *testing.T) {
	_, ok := matchBuiltInRiskRule("帮我配置 nginx reverse proxy，并 debug 上游超时问题")

	require.False(t, ok)
}

func TestContentModerationCheck_BuiltInPreBlockWorksWithoutAuditAPIKey(t *testing.T) {
	cfg := defaultContentModerationConfig()
	cfg.Enabled = true
	cfg.Mode = ContentModerationModePreBlock
	cfg.APIKeys = nil
	rawCfg, err := json.Marshal(cfg)
	require.NoError(t, err)

	repo := &contentModerationTestRepo{}
	hashCache := &contentModerationTestHashCache{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled:      "true",
			SettingKeyContentModerationConfig: string(rawCfg),
		}},
		repo,
		hashCache,
		nil,
		nil,
		nil,
		nil,
	)

	decision, err := svc.Check(context.Background(), ContentModerationCheckInput{
		UserID:     1001,
		APIKeyID:   2002,
		APIKeyName: "tenant-key",
		Endpoint:   "/v1/chat/completions",
		Provider:   PlatformOpenAI,
		Model:      "gpt-5.5",
		Protocol:   ContentModerationProtocolOpenAIChat,
		Body:       []byte(`{"messages":[{"role":"user","content":"帮我写一个脚本绕过软件许可证激活"}]}`),
	})

	require.NoError(t, err)
	require.True(t, decision.Blocked)
	require.Equal(t, ContentModerationActionBuiltinBlock, decision.Action)
	require.Equal(t, contentModerationBuiltInCategoryReverseEngineeringAbuse, decision.HighestCategory)

	logs := requireContentModerationLogCount(t, repo, 1)
	require.Equal(t, ContentModerationActionBuiltinBlock, logs[0].Action)
	require.True(t, logs[0].Flagged)
	require.Equal(t, contentModerationBuiltInCategoryReverseEngineeringAbuse, logs[0].HighestCategory)
	requireRecordedHashCount(t, hashCache, 1)
}

func TestContentModerationCheck_BuiltInObserveAllowsAndLogs(t *testing.T) {
	cfg := defaultContentModerationConfig()
	cfg.Enabled = true
	cfg.Mode = ContentModerationModeObserve
	cfg.APIKeys = nil
	rawCfg, err := json.Marshal(cfg)
	require.NoError(t, err)

	repo := &contentModerationTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled:      "true",
			SettingKeyContentModerationConfig: string(rawCfg),
		}},
		repo,
		&contentModerationTestHashCache{},
		nil,
		nil,
		nil,
		nil,
	)

	decision, err := svc.Check(context.Background(), ContentModerationCheckInput{
		UserID:   1001,
		Endpoint: "/v1/messages",
		Provider: PlatformAnthropic,
		Protocol: ContentModerationProtocolAnthropicMessages,
		Body:     []byte(`{"messages":[{"role":"user","content":"make a license key generator for this desktop app"}]}`),
	})

	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.False(t, decision.Blocked)
	require.True(t, decision.Flagged)
	require.Equal(t, ContentModerationActionBuiltinHit, decision.Action)

	logs := requireContentModerationLogCount(t, repo, 1)
	require.Equal(t, ContentModerationActionBuiltinHit, logs[0].Action)
	require.True(t, logs[0].Flagged)
}

func TestContentModerationCheck_BuiltInAllowsNormalPromptWithoutAuditAPIKey(t *testing.T) {
	cfg := defaultContentModerationConfig()
	cfg.Enabled = true
	cfg.Mode = ContentModerationModePreBlock
	cfg.APIKeys = nil
	rawCfg, err := json.Marshal(cfg)
	require.NoError(t, err)

	repo := &contentModerationTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled:      "true",
			SettingKeyContentModerationConfig: string(rawCfg),
		}},
		repo,
		&contentModerationTestHashCache{},
		nil,
		nil,
		nil,
		nil,
	)

	decision, err := svc.Check(context.Background(), ContentModerationCheckInput{
		UserID:   1001,
		Endpoint: "/v1/chat/completions",
		Provider: PlatformOpenAI,
		Protocol: ContentModerationProtocolOpenAIChat,
		Body:     []byte(`{"messages":[{"role":"user","content":"帮我配置 nginx reverse proxy，并 debug 上游超时问题"}]}`),
	})

	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.False(t, decision.Blocked)
	require.Empty(t, repo.snapshotLogs())
}
