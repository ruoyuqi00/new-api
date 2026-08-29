package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestPreserveExplicitImageDimensionsAfterChannelOverride(t *testing.T) {
	request := &dto.ImageRequest{Size: "650x1024"}
	input := []byte(`{"model":"gpt-image-2","size":"1024x1024","prompt":"test"}`)

	output, err := preserveRequestedImageDimensions(input, request)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-image-2","size":"650x1024","prompt":"test"}`, string(output))
}

func TestPreserveExplicitImageDimensionsLeavesTierAliasUnchanged(t *testing.T) {
	request := &dto.ImageRequest{Size: "2k"}
	input := []byte(`{"model":"gpt-image-2-2k","size":"2048x2048","prompt":"test"}`)

	output, err := preserveRequestedImageDimensions(input, request)
	require.NoError(t, err)
	require.JSONEq(t, string(input), string(output))
}

func TestPreserveImageTierAspectRatioAfterChannelOverride(t *testing.T) {
	tests := []struct {
		name       string
		request    string
		overridden string
		want       string
	}{
		{
			name:       "1k portrait",
			request:    `{"size":"1k","aspect_ratio":"2:3"}`,
			overridden: `{"model":"gpt-image-2","size":"1024x1024","aspect_ratio":"2:3","prompt":"test"}`,
			want:       `{"model":"gpt-image-2","size":"683x1024","aspect_ratio":"2:3","prompt":"test"}`,
		},
		{
			name:       "2k landscape",
			request:    `{"size":"2k","aspect_ratio":"3:2"}`,
			overridden: `{"model":"gpt-image-2","size":"2048x2048","aspect_ratio":"3:2","prompt":"test"}`,
			want:       `{"model":"gpt-image-2","size":"2048x1365","aspect_ratio":"3:2","prompt":"test"}`,
		},
		{
			name:       "4k portrait",
			request:    `{"size":"4k","aspect_ratio":"9:16"}`,
			overridden: `{"model":"gpt-image-2","size":"4096x4096","aspect_ratio":"9:16","prompt":"test"}`,
			want:       `{"model":"gpt-image-2","size":"2304x4096","aspect_ratio":"9:16","prompt":"test"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var request dto.ImageRequest
			require.NoError(t, common.Unmarshal([]byte(test.request), &request))

			output, err := preserveRequestedImageDimensions([]byte(test.overridden), &request)
			require.NoError(t, err)
			require.JSONEq(t, test.want, string(output))
		})
	}
}

func TestImageChannelOverrideCannotReplaceExplicitDimensions(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ParamOverride: map[string]interface{}{"size": "2048x2048"},
	}}
	request := &dto.ImageRequest{Size: "650x1024"}
	converted := []byte(`{"model":"gpt-image-2","size":"650x1024","prompt":"test"}`)

	overridden, err := relaycommon.ApplyParamOverrideWithRelayInfo(converted, info)
	require.NoError(t, err)
	output, err := preserveRequestedImageDimensions(overridden, request)
	require.NoError(t, err)
	require.JSONEq(t, string(converted), string(output))
}
