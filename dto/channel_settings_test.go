package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvancedCustomValidateResponsesToChatConverterPath(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
			},
		},
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name         string
		incomingPath string
	}{
		{name: "chat completions", incomingPath: "/v1/chat/completions"},
		{name: "responses compact", incomingPath: "/v1/responses/compact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdvancedCustomConfig{
				Routes: []AdvancedCustomRoute{
					{
						IncomingPath: tt.incomingPath,
						UpstreamPath: "/v1/chat/completions",
						Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
					},
				},
			}
			err := config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "converter does not match incoming_path")
		})
	}
}

func TestAdvancedCustomValidateModelListRouteConstraints(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: AdvancedCustomModelListPath,
				UpstreamPath: "https://upstream.example/provider/models",
				Converter:    AdvancedCustomConverterNone,
			},
		},
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name  string
		route AdvancedCustomRoute
		want  string
	}{
		{
			name: "converter is rejected",
			route: AdvancedCustomRoute{
				IncomingPath: AdvancedCustomModelListPath,
				UpstreamPath: "/provider/models",
				Converter:    AdvancedCustomConverterOpenAIChatCompletionsToOpenAIResponses,
			},
			want: "converter must be none",
		},
		{
			name: "model template is rejected",
			route: AdvancedCustomRoute{
				IncomingPath: AdvancedCustomModelListPath,
				UpstreamPath: "/provider/models/{model}",
				Converter:    AdvancedCustomConverterNone,
			},
			want: "upstream_path must not contain {model}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&AdvancedCustomConfig{Routes: []AdvancedCustomRoute{tt.route}}).Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}
