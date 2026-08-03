package helper

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureActualResponseModelJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "chat completion top-level model", data: `{"model":"gpt-upstream"}`, want: "gpt-upstream"},
		{name: "responses event model", data: `{"type":"response.completed","response":{"model":"gpt-response"}}`, want: "gpt-response"},
		{name: "claude message start model", data: `{"type":"message_start","message":{"model":"claude-upstream"}}`, want: "claude-upstream"},
		{name: "realtime session model", data: `{"type":"session.created","session":{"model":"gpt-realtime"}}`, want: "gpt-realtime"},
		{name: "response model takes precedence", data: `{"model":"wrapper-model","response":{"model":"response-model"}}`, want: "response-model"},
		{name: "missing model", data: `{"type":"response.output_text.delta","delta":"hello"}`, want: ""},
		{name: "invalid json", data: `not-json`, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &common.RelayInfo{
				ChannelMeta: &common.ChannelMeta{UpstreamModelName: "forwarded-model"},
			}

			CaptureActualResponseModelJSON(info, []byte(tt.data))

			assert.Equal(t, "forwarded-model", info.ForwardedModelName)
			assert.Equal(t, tt.want, info.ActualResponseModel)
		})
	}
}

func TestCaptureActualResponseModelJSONKeepsFirstModelAndLimitsLength(t *testing.T) {
	info := &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{UpstreamModelName: "forwarded-model"},
	}
	longModel := strings.Repeat("模", 101)

	CaptureActualResponseModelJSON(info, []byte(`{"model":"`+longModel+`"}`))
	CaptureActualResponseModelJSON(info, []byte(`{"model":"later-model"}`))

	assert.Equal(t, 100, len([]rune(info.ActualResponseModel)))
	assert.Equal(t, strings.Repeat("模", 100), info.ActualResponseModel)
}

func TestCaptureActualResponseModelJSONRunsBeforeClientNormalization(t *testing.T) {
	info := &common.RelayInfo{
		OriginModelName: "public-model",
		ChannelMeta: &common.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "private-model",
		},
	}
	data := []byte(`{"model":"private-model"}`)

	CaptureActualResponseModelJSON(info, data)
	got, changed, err := NormalizeClientResponseModelJSON(info, data)

	require.NoError(t, err)
	assert.True(t, changed)
	assert.JSONEq(t, `{"model":"public-model"}`, string(got))
	assert.Equal(t, "private-model", info.ActualResponseModel)
}

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
