package helper

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMaxTokensBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newJSONContext := func(body string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/relay", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		return c
	}

	const huge = "18446744073686646784"

	t.Run("openai max_tokens overflow rejected", func(t *testing.T) {
		c := newJSONContext(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"max_tokens":` + huge + `}`)
		_, err := GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_tokens is invalid")
	})

	t.Run("openai max_completion_tokens overflow rejected", func(t *testing.T) {
		c := newJSONContext(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"max_completion_tokens":` + huge + `}`)
		_, err := GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_tokens is invalid")
	})

	t.Run("openai normal max_completion_tokens accepted", func(t *testing.T) {
		c := newJSONContext(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"max_completion_tokens":8192}`)
		req, err := GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
		require.NoError(t, err)
		require.EqualValues(t, 8192, *req.MaxCompletionTokens)
	})

	t.Run("claude max_tokens overflow rejected", func(t *testing.T) {
		c := newJSONContext(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hi"}],"max_tokens":` + huge + `}`)
		_, err := GetAndValidateClaudeRequest(c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_tokens is invalid")
	})

	t.Run("claude max_tokens_to_sample overflow rejected", func(t *testing.T) {
		c := newJSONContext(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hi"}],"max_tokens_to_sample":` + huge + `}`)
		_, err := GetAndValidateClaudeRequest(c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_tokens is invalid")
	})

	t.Run("claude normal max_tokens accepted", func(t *testing.T) {
		c := newJSONContext(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hi"}],"max_tokens":8192}`)
		req, err := GetAndValidateClaudeRequest(c)
		require.NoError(t, err)
		require.EqualValues(t, 8192, *req.MaxTokens)
	})

	t.Run("gemini maxOutputTokens overflow rejected", func(t *testing.T) {
		c := newJSONContext(`{"contents":[{"parts":[{"text":"hi"}]}],"generationConfig":{"maxOutputTokens":` + huge + `}}`)
		_, err := GetAndValidateGeminiRequest(c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "maxOutputTokens is invalid")
	})

	t.Run("gemini normal maxOutputTokens accepted", func(t *testing.T) {
		c := newJSONContext(`{"contents":[{"parts":[{"text":"hi"}]}],"generationConfig":{"maxOutputTokens":8192}}`)
		req, err := GetAndValidateGeminiRequest(c)
		require.NoError(t, err)
		require.EqualValues(t, 8192, *req.GenerationConfig.MaxOutputTokens)
	})

	t.Run("responses max_output_tokens overflow rejected", func(t *testing.T) {
		c := newJSONContext(`{"model":"gpt-4o","input":"hi","max_output_tokens":` + huge + `}`)
		_, err := GetAndValidateResponsesRequest(c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_output_tokens is invalid")
	})

	t.Run("responses normal max_output_tokens accepted", func(t *testing.T) {
		c := newJSONContext(`{"model":"gpt-4o","input":"hi","max_output_tokens":8192}`)
		req, err := GetAndValidateResponsesRequest(c)
		require.NoError(t, err)
		require.EqualValues(t, 8192, *req.MaxOutputTokens)
	})
}
