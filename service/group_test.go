package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserGroupRatioForUserFallsBackToExistingGroupResolution(t *testing.T) {
	originalGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	originalUserGroupRatio := ratio_setting.UserGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatio))
		require.NoError(t, ratio_setting.UpdateUserGroupRatioByJSONString(originalUserGroupRatio))
	})

	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"china":0.8}}`))
	require.NoError(t, ratio_setting.UpdateUserGroupRatioByJSONString(`{"81":{"china":0.42}}`))

	assert.Equal(t, 0.42, GetUserGroupRatioForUser(81, "vip", "china"))
	assert.Equal(t, 0.8, GetUserGroupRatioForUser(82, "vip", "china"))
	assert.Equal(t, 1.0, GetUserGroupRatioForUser(82, "vip", "standard"))
}
