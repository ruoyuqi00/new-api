package types

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPIErrorPublicProjectionKeepsInternalErrorPrivate(t *testing.T) {
	privateMessage := "POST https://private-upstream.example/v1 IP 10.0.0.8 channel secret-channel Authorization Bearer sk-private raw-body"
	apiError := NewOpenAIError(
		errors.New(privateMessage),
		ErrorCodeBadResponseStatusCode,
		502,
		ErrOptionWithPublicError(OpenAIError{
			Message: "The upstream service is temporarily unavailable.",
			Type:    "upstream_error",
			Code:    ErrorCodeBadResponseStatusCode,
		}),
	)

	require.Equal(t, privateMessage, apiError.Error())
	publicOpenAI := apiError.ToPublicOpenAIError("req-public")
	assert.Equal(t, "upstream_error", publicOpenAI.Type)
	assert.Equal(t, ErrorCodeBadResponseStatusCode, publicOpenAI.Code)
	assert.Contains(t, publicOpenAI.Message, "req-public")
	assert.NotContains(t, publicOpenAI.Message, "private-upstream")
	assert.NotContains(t, publicOpenAI.Message, "10.0.0.8")
	assert.NotContains(t, publicOpenAI.Message, "secret-channel")
	assert.NotContains(t, publicOpenAI.Message, "sk-private")
	assert.NotContains(t, publicOpenAI.Message, "raw-body")

	publicClaude := apiError.ToPublicClaudeError("req-public")
	assert.Equal(t, "upstream_error", publicClaude.Type)
	assert.Equal(t, publicOpenAI.Message, publicClaude.Message)
}

func TestNewAPIErrorPublicProjectionPreservesLocalValidationMessage(t *testing.T) {
	apiError := NewErrorWithStatusCode(errors.New("model is required"), ErrorCodeInvalidRequest, 400)

	public := apiError.ToPublicOpenAIError("req-local")
	assert.Contains(t, public.Message, "model is required")
	assert.Contains(t, public.Message, "req-local")
}
