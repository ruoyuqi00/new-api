package service

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreConsumeBillingRejectsNegativeQuota(t *testing.T) {
	apiErr := PreConsumeBilling(nil, -1, nil)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, types.ErrorCodeModelPriceError, apiErr.GetErrorCode())
	assert.True(t, types.IsSkipRetryError(apiErr))
}
