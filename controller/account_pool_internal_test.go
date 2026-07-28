package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderAccountViewIncludesCachedEntitlementAndUsage(t *testing.T) {
	account := model.ProviderAccount{
		Metadata:       `{"plan_type":"team"}`,
		UsageSnapshot:  `{"plan_type":"pro","rate_limit":{"primary_window":{"used_percent":28.5},"secondary_window":{"used_percent":61}}}`,
		UsageUpdatedAt: 1_720_000_000,
	}

	view := providerAccountViewFromModel(account)

	require.NotNil(t, view.PrimaryUsage)
	require.NotNil(t, view.SecondaryUsage)
	assert.Equal(t, "pro", view.PlanType)
	assert.Equal(t, 28.5, *view.PrimaryUsage)
	assert.Equal(t, 61.0, *view.SecondaryUsage)
	assert.Equal(t, int64(1_720_000_000), view.UsageUpdatedAt)
}

func TestProviderAccountViewUsesImportedPlanBeforeUsageRefresh(t *testing.T) {
	view := providerAccountViewFromModel(model.ProviderAccount{
		Metadata: `{"plan_type":"plus"}`,
	})

	assert.Equal(t, "plus", view.PlanType)
	assert.Nil(t, view.PrimaryUsage)
	assert.Nil(t, view.SecondaryUsage)
}
