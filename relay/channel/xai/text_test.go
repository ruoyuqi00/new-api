package xai

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestXAIStreamChunkHasSignalRejectsEmptyChunk(t *testing.T) {
	require.False(t, xAIStreamChunkHasSignal(&dto.ChatCompletionsStreamResponse{}))
}

func TestXAIStreamChunkHasSignalAcceptsContent(t *testing.T) {
	content := "ok"
	require.True(t, xAIStreamChunkHasSignal(&dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &content}},
		},
	}))
}

func TestXAITextResponseHasSignalRejectsEmptyResponse(t *testing.T) {
	require.False(t, xAITextResponseHasSignal(ChatCompletionResponse{}))
}

func TestXAITextResponseHasSignalAcceptsContentWithoutUsage(t *testing.T) {
	resp := ChatCompletionResponse{
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{Role: "assistant", Content: "ok"},
			},
		},
	}

	require.True(t, xAITextResponseHasSignal(resp))
	require.Equal(t, "ok", xAIResponseText(resp))
}
