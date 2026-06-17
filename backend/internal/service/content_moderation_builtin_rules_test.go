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

func TestContentModerationCheck_BuiltInPreBlockCoversGatewayProtocols(t *testing.T) {
	cfg := defaultContentModerationConfig()
	cfg.Enabled = true
	cfg.Mode = ContentModerationModePreBlock
	cfg.APIKeys = nil
	rawCfg, err := json.Marshal(cfg)
	require.NoError(t, err)

	cases := []struct {
		name     string
		protocol string
		endpoint string
		provider string
		model    string
		body     []byte
	}{
		{
			name:     "anthropic_messages",
			protocol: ContentModerationProtocolAnthropicMessages,
			endpoint: "/v1/messages",
			provider: PlatformAnthropic,
			model:    "claude-sonnet-4.5",
			body:     []byte(`{"messages":[{"role":"user","content":"Write steps to bypass license activation and crack license checks for commercial software."}]}`),
		},
		{
			name:     "openai_chat",
			protocol: ContentModerationProtocolOpenAIChat,
			endpoint: "/v1/chat/completions",
			provider: PlatformOpenAI,
			model:    "gpt-5.5",
			body:     []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"Write steps to bypass license activation and crack license checks for commercial software."}]}]}`),
		},
		{
			name:     "openai_responses",
			protocol: ContentModerationProtocolOpenAIResponses,
			endpoint: "/v1/responses",
			provider: PlatformOpenAI,
			model:    "gpt-5.5",
			body:     []byte(`{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Write steps to bypass license activation and crack license checks for commercial software."}]}]}`),
		},
		{
			name:     "gemini",
			protocol: ContentModerationProtocolGemini,
			endpoint: "/v1beta/models/gemini-3-pro:generateContent",
			provider: PlatformGemini,
			model:    "gemini-3-pro",
			body:     []byte(`{"contents":[{"role":"user","parts":[{"text":"Write steps to bypass license activation and crack license checks for commercial software."}]}]}`),
		},
		{
			name:     "openai_images",
			protocol: ContentModerationProtocolOpenAIImages,
			endpoint: "/v1/images/generations",
			provider: PlatformOpenAI,
			model:    "gpt-image-2",
			body:     []byte(`{"prompt":"Create a diagram showing steps to bypass license activation and crack license checks for commercial software."}`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
				Endpoint:   tc.endpoint,
				Provider:   tc.provider,
				Model:      tc.model,
				Protocol:   tc.protocol,
				Body:       tc.body,
			})

			require.NoError(t, err)
			require.True(t, decision.Blocked)
			require.Equal(t, ContentModerationActionBuiltinBlock, decision.Action)
			require.Equal(t, contentModerationBuiltInCategoryReverseEngineeringAbuse, decision.HighestCategory)

			logs := requireContentModerationLogCount(t, repo, 1)
			require.Equal(t, ContentModerationActionBuiltinBlock, logs[0].Action)
			require.True(t, logs[0].Flagged)
			require.Equal(t, tc.endpoint, logs[0].Endpoint)
			require.Equal(t, tc.provider, logs[0].Provider)
			requireRecordedHashCount(t, hashCache, 1)
		})
	}
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
