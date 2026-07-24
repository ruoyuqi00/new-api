package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientResponseModelNameUsesOriginOnlyForMappedRequests(t *testing.T) {
	tests := []struct {
		name string
		info *common.RelayInfo
		want string
	}{
		{
			name: "mapped request returns origin model",
			info: &common.RelayInfo{
				OriginModelName: "public-model",
				ChannelMeta: &common.ChannelMeta{
					IsModelMapped:     true,
					UpstreamModelName: "private-model",
				},
			},
			want: "public-model",
		},
		{
			name: "unmapped request returns upstream model",
			info: &common.RelayInfo{
				OriginModelName: "public-model",
				ChannelMeta: &common.ChannelMeta{
					UpstreamModelName: "private-model",
				},
			},
			want: "private-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.info.ClientResponseModelName())
		})
	}
}

func TestNormalizeClientResponseModelJSONRewritesOnlySupportedModelFields(t *testing.T) {
	info := &common.RelayInfo{
		OriginModelName: "public-model",
		ChannelMeta: &common.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "private-model",
		},
	}
	data := []byte(`{"model":"private-model","message":{"model":"message-model"},"response":{"model":"private-model","metadata":{"model":"nested-model"}}}`)

	got, changed, err := NormalizeClientResponseModelJSON(info, data)

	require.NoError(t, err)
	assert.True(t, changed)
	assert.JSONEq(t, `{"model":"public-model","message":{"model":"message-model"},"response":{"model":"public-model","metadata":{"model":"nested-model"}}}`, string(got))
}

func TestNormalizeClientResponseModelJSONLeavesUnmappedAndNonJSONDataUntouched(t *testing.T) {
	unmappedInfo := &common.RelayInfo{
		OriginModelName: "public-model",
		ChannelMeta: &common.ChannelMeta{
			UpstreamModelName: "private-model",
		},
	}
	mappedInfo := &common.RelayInfo{
		OriginModelName: "public-model",
		ChannelMeta: &common.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "private-model",
		},
	}

	for _, tt := range []struct {
		info *common.RelayInfo
		data []byte
	}{
		{info: unmappedInfo, data: []byte(`{"model":"private-model"}`)},
		{info: mappedInfo, data: []byte("data: not-json\n\n")},
	} {
		got, changed, err := NormalizeClientResponseModelJSON(tt.info, tt.data)

		require.NoError(t, err)
		assert.False(t, changed)
		assert.Equal(t, tt.data, got)
	}
}

func TestNormalizeClientResponseModelJSONLeavesMappedRequestWithoutPublicModelUntouched(t *testing.T) {
	info := &common.RelayInfo{
		OriginModelName: " \t",
		ChannelMeta: &common.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "private-model",
		},
	}
	data := []byte(`{"model":"other-model"}`)

	got, changed, err := NormalizeClientResponseModelJSON(info, data)

	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, data, got)
}
