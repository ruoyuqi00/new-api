//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type openAICompatibleGroupRepoStub struct {
	groupRepoNoop
	groups map[int64]*Group
}

func (s *openAICompatibleGroupRepoStub) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	return s.groups[id], nil
}

func TestAdminServiceApplyOpenAICompatibleGroupModelMapping_ReplacesDefaultOpenAISelfMapping(t *testing.T) {
	svc := &adminServiceImpl{
		groupRepo: &openAICompatibleGroupRepoStub{groups: map[int64]*Group{
			17: {
				ID:       17,
				Platform: PlatformOpenAI,
				ModelsListConfig: GroupModelsListConfig{Models: []string{
					"grok-3-beta",
					"grok-3-fast-beta",
					"grok-3-beta",
				}},
			},
		}},
	}
	account := &Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"api_key": "redacted",
			"model_mapping": map[string]any{
				"gpt-5.5": "gpt-5.5",
				"gpt-5.4": "gpt-5.4",
				"gpt-4o":  "gpt-4o",
			},
		},
	}

	err := svc.applyOpenAICompatibleGroupModelMapping(context.Background(), account, []int64{17})

	require.NoError(t, err)
	require.Equal(t, "redacted", account.Credentials["api_key"])
	require.Equal(t, map[string]string{
		"grok-3-beta":      "grok-3-beta",
		"grok-3-fast-beta": "grok-3-fast-beta",
	}, stringMappingFromRaw(account.Credentials["model_mapping"]))
}

func TestAdminServiceApplyOpenAICompatibleGroupModelMapping_FillsEmptyMappingFromGroupModels(t *testing.T) {
	svc := &adminServiceImpl{
		groupRepo: &openAICompatibleGroupRepoStub{groups: map[int64]*Group{
			18: {
				ID:       18,
				Platform: PlatformOpenAI,
				ModelsListConfig: GroupModelsListConfig{Models: []string{
					"gemini-2.5-pro",
					" gemini-2.5-flash ",
					"",
				}},
			},
		}},
	}
	account := &Account{
		Platform:    PlatformOpenAI,
		Credentials: map[string]any{"base_url": "https://example.invalid/v1"},
	}

	err := svc.applyOpenAICompatibleGroupModelMapping(context.Background(), account, []int64{18})

	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"gemini-2.5-pro":   "gemini-2.5-pro",
		"gemini-2.5-flash": "gemini-2.5-flash",
	}, stringMappingFromRaw(account.Credentials["model_mapping"]))
}

func TestAdminServiceApplyOpenAICompatibleGroupModelMapping_ReplacesOpenAISelfMappingForNonOpenAIGroupModels(t *testing.T) {
	svc := &adminServiceImpl{
		groupRepo: &openAICompatibleGroupRepoStub{groups: map[int64]*Group{
			17: {
				ID:               17,
				Platform:         PlatformOpenAI,
				ModelsListConfig: GroupModelsListConfig{Models: []string{"grok-3-beta"}},
			},
		}},
	}
	account := &Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"gpt-4o": "gpt-4o"},
		},
	}

	err := svc.applyOpenAICompatibleGroupModelMapping(context.Background(), account, []int64{17})

	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"grok-3-beta": "grok-3-beta",
	}, stringMappingFromRaw(account.Credentials["model_mapping"]))
}

func TestAdminServiceApplyOpenAICompatibleGroupModelMapping_PreservesCustomMapping(t *testing.T) {
	svc := &adminServiceImpl{
		groupRepo: &openAICompatibleGroupRepoStub{groups: map[int64]*Group{
			17: {
				ID:               17,
				Platform:         PlatformOpenAI,
				ModelsListConfig: GroupModelsListConfig{Models: []string{"grok-3-beta"}},
			},
		}},
	}
	account := &Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"custom-model": "upstream-model"},
		},
	}

	err := svc.applyOpenAICompatibleGroupModelMapping(context.Background(), account, []int64{17})

	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"custom-model": "upstream-model",
	}, stringMappingFromRaw(account.Credentials["model_mapping"]))
}

func TestAdminServiceApplyOpenAICompatibleGroupModelMapping_IgnoresNonOpenAIAccounts(t *testing.T) {
	svc := &adminServiceImpl{
		groupRepo: &openAICompatibleGroupRepoStub{groups: map[int64]*Group{
			17: {
				ID:               17,
				Platform:         PlatformOpenAI,
				ModelsListConfig: GroupModelsListConfig{Models: []string{"grok-3-beta"}},
			},
		}},
	}
	account := &Account{Platform: PlatformGemini, Credentials: map[string]any{}}

	err := svc.applyOpenAICompatibleGroupModelMapping(context.Background(), account, []int64{17})

	require.NoError(t, err)
	require.Nil(t, account.Credentials["model_mapping"])
}

func TestIsDefaultOpenAIModelMapping(t *testing.T) {
	require.True(t, isDefaultOpenAIModelMapping(map[string]string{
		"gpt-5.5": "gpt-5.5",
		"gpt-5.4": "gpt-5.4",
		"gpt-4o":  "gpt-4o",
	}))
	require.False(t, isDefaultOpenAIModelMapping(map[string]string{
		"grok-3-beta": "grok-3-beta",
	}))
	require.False(t, isDefaultOpenAIModelMapping(map[string]string{
		"gpt-5.5": "different-upstream",
	}))
}
