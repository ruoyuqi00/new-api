package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestPreserveExplicitImageDimensionsAfterChannelOverride(t *testing.T) {
	request := &dto.ImageRequest{Size: "650x1024"}
	input := []byte(`{"model":"gpt-image-2","size":"1024x1024","prompt":"test"}`)

	output, err := preserveExplicitImageDimensions(input, request)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-image-2","size":"650x1024","prompt":"test"}`, string(output))
}

func TestPreserveExplicitImageDimensionsLeavesTierAliasUnchanged(t *testing.T) {
	request := &dto.ImageRequest{Size: "2k"}
	input := []byte(`{"model":"gpt-image-2-2k","size":"2048x2048","prompt":"test"}`)

	output, err := preserveExplicitImageDimensions(input, request)
	require.NoError(t, err)
	require.JSONEq(t, string(input), string(output))
}

func TestImageChannelOverrideCannotReplaceExplicitDimensions(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ParamOverride: map[string]interface{}{"size": "2048x2048"},
	}}
	request := &dto.ImageRequest{Size: "650x1024"}
	converted := []byte(`{"model":"gpt-image-2","size":"650x1024","prompt":"test"}`)

	overridden, err := relaycommon.ApplyParamOverrideWithRelayInfo(converted, info)
	require.NoError(t, err)
	output, err := preserveExplicitImageDimensions(overridden, request)
	require.NoError(t, err)
	require.JSONEq(t, string(converted), string(output))
}
