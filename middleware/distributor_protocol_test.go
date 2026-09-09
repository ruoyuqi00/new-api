package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestChannelSupportsRequestPathOnlyChecksPathCapability(t *testing.T) {
	assert.True(t, channelSupportsRequestPath(
		&model.Channel{Type: constant.ChannelTypeAnthropic},
		"/v1/chat/completions",
	))
	assert.True(t, channelSupportsRequestPath(
		&model.Channel{Type: constant.ChannelTypeOpenAI},
		"/v1/messages",
	))
	assert.True(t, channelSupportsRequestPath(
		&model.Channel{Type: constant.ChannelTypeOpenAI},
		"/v1/chat/completions",
	))
	assert.True(t, channelSupportsRequestPath(
		&model.Channel{Type: constant.ChannelTypeAnthropic},
		"/v1/messages",
	))
}
