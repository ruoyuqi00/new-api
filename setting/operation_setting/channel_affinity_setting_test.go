package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultCodexAffinityUsesEditorSupportedKeySourceTypes(t *testing.T) {
	setting := GetChannelAffinitySetting()
	require.NotNil(t, setting)

	var codexRule *ChannelAffinityRule
	for index := range setting.Rules {
		if setting.Rules[index].Name == "codex cli trace" {
			codexRule = &setting.Rules[index]
			break
		}
	}
	require.NotNil(t, codexRule)

	supportedTypes := map[string]bool{
		"context_int":    true,
		"context_string": true,
		"request_header": true,
		"gjson":          true,
	}
	contextKeys := map[string]bool{}
	for _, source := range codexRule.KeySources {
		require.Truef(t, supportedTypes[source.Type], "unsupported key source type %q", source.Type)
		if source.Type == "context_string" {
			contextKeys[source.Key] = true
		}
	}
	require.True(t, contextKeys[ChannelAffinityConversationContextKey])
	require.True(t, contextKeys[ChannelAffinityResponseChainContextKey])
}
