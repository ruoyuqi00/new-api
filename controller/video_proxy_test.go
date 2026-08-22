package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXAIVideoProxyAuthorizationStaysOnConfiguredOrigin(t *testing.T) {
	baseURL := "https://provider.example/api"
	channel := &model.Channel{Type: constant.ChannelTypeXai, Key: "legacy-channel-key", BaseURL: &baseURL}
	task := &model.Task{PrivateData: model.TaskPrivateData{Key: "selected-task-key"}}

	header, err := xAIVideoProxyAuthorization(channel, task, "https://provider.example/api/v1/videos/result/content")
	require.NoError(t, err)
	assert.Equal(t, "Bearer selected-task-key", header)

	header, err = xAIVideoProxyAuthorization(channel, task, "https://cdn.example/video.mp4")
	require.NoError(t, err)
	assert.Empty(t, header)

	header, err = xAIVideoProxyAuthorization(channel, task, "https://provider.example/apiv2/video.mp4")
	require.NoError(t, err)
	assert.Empty(t, header)
}

func TestXAIVideoProxyAuthorizationSupportsLegacySingleKeyOnly(t *testing.T) {
	baseURL := "https://provider.example"
	task := &model.Task{}
	legacy := &model.Channel{Type: constant.ChannelTypeXai, Key: "legacy-channel-key", BaseURL: &baseURL}

	header, err := xAIVideoProxyAuthorization(legacy, task, "https://provider.example/v1/videos/result/content")
	require.NoError(t, err)
	assert.Equal(t, "Bearer legacy-channel-key", header)

	multiKey := *legacy
	multiKey.ChannelInfo.IsMultiKey = true
	header, err = xAIVideoProxyAuthorization(&multiKey, task, "https://provider.example/v1/videos/result/content")
	require.Error(t, err)
	assert.Empty(t, header)
}
