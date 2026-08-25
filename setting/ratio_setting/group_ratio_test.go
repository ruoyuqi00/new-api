package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserGroupRatioUsesUserOverrideBeforeGroupRules(t *testing.T) {
	originalGroupGroupRatio := GroupGroupRatio2JSONString()
	originalUserGroupRatio := UserGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupGroupRatioByJSONString(originalGroupGroupRatio))
		require.NoError(t, UpdateUserGroupRatioByJSONString(originalUserGroupRatio))
	})

	require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"vip":{"china":0.8}}`))
	require.NoError(t, UpdateUserGroupRatioByJSONString(`{"81":{"china":0.42}}`))

	ratio, ok := GetUserGroupRatio(81, "vip", "china")
	require.True(t, ok)
	assert.Equal(t, 0.42, ratio)

	ratio, ok = GetUserGroupRatio(82, "vip", "china")
	require.True(t, ok)
	assert.Equal(t, 0.8, ratio)

	ratio, ok = GetUserGroupRatio(83, "vip", "standard")
	assert.False(t, ok)
	assert.Equal(t, -1.0, ratio)

	groupRatio, ok := GetGroupGroupRatio("vip", "china")
	require.True(t, ok)
	assert.Equal(t, 0.8, groupRatio)
}

func TestUpdateUserGroupRatioRejectsInvalidEntriesWithoutMutation(t *testing.T) {
	original := UserGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateUserGroupRatioByJSONString(original))
	})

	for _, value := range []string{
		`{"81":{"china":-0.1}}`,
		`{"81":{"china":null}}`,
		`{"":{"china":0.3}}`,
		`{"81":{"":0.3}}`,
	} {
		assert.Error(t, UpdateUserGroupRatioByJSONString(value), value)
		assert.Equal(t, original, UserGroupRatio2JSONString(), value)
	}
}
