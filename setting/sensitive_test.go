package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldCheckPromptSensitiveForGroup(t *testing.T) {
	originalGroups := SensitiveInputCheckGroups2JSONString()
	originalEnabled := CheckSensitiveEnabled
	originalPromptEnabled := CheckSensitiveOnPromptEnabled
	t.Cleanup(func() {
		require.NoError(t, UpdateSensitiveInputCheckGroupsByJSONString(originalGroups))
		CheckSensitiveEnabled = originalEnabled
		CheckSensitiveOnPromptEnabled = originalPromptEnabled
	})

	CheckSensitiveEnabled = true
	CheckSensitiveOnPromptEnabled = true
	require.NoError(t, UpdateSensitiveInputCheckGroupsByJSONString(`{"restricted":true,"open":false}`))

	t.Run("explicitly enabled group is checked", func(t *testing.T) {
		assert.True(t, ShouldCheckPromptSensitiveForGroup("restricted"))
	})

	t.Run("explicitly disabled group is exempt", func(t *testing.T) {
		assert.False(t, ShouldCheckPromptSensitiveForGroup("open"))
	})

	t.Run("missing group remains checked for backward compatibility", func(t *testing.T) {
		assert.True(t, ShouldCheckPromptSensitiveForGroup("legacy"))
	})

	t.Run("global switch remains the outer gate", func(t *testing.T) {
		CheckSensitiveEnabled = false
		assert.False(t, ShouldCheckPromptSensitiveForGroup("restricted"))
	})
}

func TestUpdateSensitiveInputRetentionDays(t *testing.T) {
	original := SensitiveInputRetentionDays
	t.Cleanup(func() { SensitiveInputRetentionDays = original })

	require.NoError(t, UpdateSensitiveInputRetentionDays("7"))
	assert.Equal(t, 7, SensitiveInputRetentionDays)
	require.NoError(t, UpdateSensitiveInputRetentionDays("1"))
	assert.Equal(t, 1, SensitiveInputRetentionDays)
	require.NoError(t, UpdateSensitiveInputRetentionDays("365"))
	assert.Equal(t, 365, SensitiveInputRetentionDays)

	for _, invalid := range []string{"0", "366", "invalid"} {
		t.Run(invalid, func(t *testing.T) {
			before := SensitiveInputRetentionDays
			assert.Error(t, UpdateSensitiveInputRetentionDays(invalid))
			assert.Equal(t, before, SensitiveInputRetentionDays)
		})
	}
}
