package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	inputModerationEndpoint         = "https://api.openai.com/v1/moderations"
	defaultInputModerationModel     = "omni-moderation-latest"
	defaultInputModerationTimeout   = 3 * time.Second
	maxInputModerationResponseBytes = 1 << 20
)

type InputModerationResult struct {
	Flagged    bool
	Model      string
	Categories []string
}

type inputModerationChecker struct {
	endpoint   string
	apiKey     string
	model      string
	timeout    time.Duration
	httpClient *http.Client
}

type inputModerationRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type inputModerationResponse struct {
	Model   string `json:"model"`
	Results []struct {
		Flagged    bool            `json:"flagged"`
		Categories map[string]bool `json:"categories"`
	} `json:"results"`
}

func InputModerationEnabled() bool {
	return common.GetEnvOrDefaultBool("INPUT_MODERATION_ENABLED", false)
}

func CheckInputModeration(ctx context.Context, input string) (InputModerationResult, error) {
	timeoutSeconds := common.GetEnvOrDefault("INPUT_MODERATION_TIMEOUT_SECONDS", int(defaultInputModerationTimeout/time.Second))
	if timeoutSeconds <= 0 {
		timeoutSeconds = int(defaultInputModerationTimeout / time.Second)
	}
	checker := inputModerationChecker{
		endpoint: inputModerationEndpoint,
		apiKey:   strings.TrimSpace(os.Getenv("INPUT_MODERATION_API_KEY")),
		model: strings.TrimSpace(common.GetEnvOrDefaultString(
			"INPUT_MODERATION_MODEL",
			defaultInputModerationModel,
		)),
		timeout: time.Duration(timeoutSeconds) * time.Second,
		httpClient: &http.Client{
			Transport: http.DefaultTransport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	return checker.Check(ctx, input)
}

func (c inputModerationChecker) Check(ctx context.Context, input string) (InputModerationResult, error) {
	if strings.TrimSpace(input) == "" {
		return InputModerationResult{}, nil
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return InputModerationResult{}, fmt.Errorf("input moderation API key is not configured")
	}
	if c.endpoint == "" {
		return InputModerationResult{}, fmt.Errorf("input moderation endpoint is not configured")
	}
	if c.model == "" {
		c.model = defaultInputModerationModel
	}
	if c.timeout <= 0 {
		c.timeout = defaultInputModerationTimeout
	}
	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}

	body, err := common.Marshal(inputModerationRequest{Model: c.model, Input: input})
	if err != nil {
		return InputModerationResult{}, fmt.Errorf("encode input moderation request: %w", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return InputModerationResult{}, fmt.Errorf("create input moderation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return InputModerationResult{}, fmt.Errorf("input moderation request failed: %w", err)
	}
	defer CloseResponseBodyGracefully(resp)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return InputModerationResult{}, fmt.Errorf("input moderation returned status %d", resp.StatusCode)
	}

	var decoded inputModerationResponse
	if err := common.DecodeJson(io.LimitReader(resp.Body, maxInputModerationResponseBytes), &decoded); err != nil {
		return InputModerationResult{}, fmt.Errorf("decode input moderation response: %w", err)
	}
	if len(decoded.Results) == 0 {
		return InputModerationResult{}, fmt.Errorf("input moderation returned no results")
	}

	result := InputModerationResult{
		Flagged: decoded.Results[0].Flagged,
		Model:   decoded.Model,
	}
	if result.Model == "" {
		result.Model = c.model
	}
	for category, flagged := range decoded.Results[0].Categories {
		if flagged {
			result.Categories = append(result.Categories, category)
		}
	}
	sort.Strings(result.Categories)
	return result, nil
}
