package billing_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePerCallExpressionAllowsContextTiers(t *testing.T) {
	err := ValidateExpressionMode(
		BillingModePerCallExpr,
		`len <= 128000 ? tier("short", 0.05) : tier("long", 0.1)`,
	)

	require.NoError(t, err)
}

func TestValidatePerCallExpressionRejectsTokenPricingVariables(t *testing.T) {
	for _, expr := range []string{
		`tier("input", p * 0.05)`,
		`tier("output", c * 0.05)`,
		`tier("cache", cr * 0.05)`,
	} {
		t.Run(expr, func(t *testing.T) {
			err := ValidateExpressionMode(BillingModePerCallExpr, expr)

			require.Error(t, err)
			require.Contains(t, err.Error(), "use len for context tiers")
		})
	}
}

func TestValidateTieredExpressionKeepsTokenPricingVariables(t *testing.T) {
	require.NoError(t, ValidateExpressionMode(
		BillingModeTieredExpr,
		`tier("base", p * 2.1 + c * 8.4 + cr * 0.42)`,
	))
}
