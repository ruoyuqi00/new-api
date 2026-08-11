package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJsonRawMessageToString(t *testing.T) {
	tests := []struct {
		name string
		data json.RawMessage
		want string
	}{
		{
			name: "object",
			data: json.RawMessage(`{"city":"Paris","days":0,"strict":false}`),
			want: `{"city":"Paris","days":0,"strict":false}`,
		},
		{
			name: "string",
			data: json.RawMessage(`"{\"city\":\"Paris\",\"days\":0,\"strict\":false}"`),
			want: `{"city":"Paris","days":0,"strict":false}`,
		},
		{
			name: "null",
			data: json.RawMessage(`null`),
			want: "",
		},
		{
			name: "empty",
			data: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, JsonRawMessageToString(tt.data))
		})
	}
}

func TestValidateJSONTopLevelObjectUniqueKeys(t *testing.T) {
	require.NoError(t, ValidateJSONTopLevelObjectUniqueKeys([]byte(`{"first":{"nested":1},"second":2}`)))
	require.NoError(t, ValidateJSONTopLevelObjectUniqueKeys([]byte(`[{"model":"first"},{"model":"first"}]`)))
	require.NoError(t, ValidateJSONTopLevelObjectUniqueKeys([]byte(`{"first":{"nested":1,"nested":2}}`)))

	err := ValidateJSONTopLevelObjectUniqueKeys([]byte(`{"first":{"value":1},"first":{"value":2}}`))
	require.Error(t, err)
	var duplicate *DuplicateJSONTopLevelKeyError
	require.ErrorAs(t, err, &duplicate)
	assert.Equal(t, "first", duplicate.Key)
}
