# Responses Incomplete Stream Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert accepted `/v1/responses` streams that end without a terminal event into one structured `response.failed` event without retrying or refunding the request.

**Architecture:** Keep protocol completion knowledge in the OpenAI Responses adapter. Reuse the shared stream status for transport cause reporting, return downstream write errors to the scanner, and bind the upstream HTTP request to the incoming request context. The accepted-stream handler continues returning a nil relay error so existing settlement runs exactly once.

**Tech Stack:** Go 1.22+, Gin, net/http, SSE, testify

---

### Task 1: Reproduce an accepted stream ending without a terminal event

**Files:**
- Modify: `relay/channel/openai/relay_responses_test.go`
- Test: `relay/channel/openai/relay_responses_test.go`

- [ ] **Step 1: Write the failing EOF regression test**

Add a test that supplies valid Responses events but no terminal event or `[DONE]`:

```go
func TestOaiResponsesStreamHandlerEmitsFailureForEOFWithoutTerminalEvent(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })

	ctx, recorder := clientResponseTestContext()
	ctx.Request.URL.Path = "/v1/responses"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"model\":\"upstream-model\",\"status\":\"in_progress\"}}\n\n" +
				"data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n",
		)),
	}

	usage, relayErr := OaiResponsesStreamHandler(ctx, mappedClientResponseInfo(), resp)

	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "event: response.failed"))
	require.Contains(t, recorder.Body.String(), `"code":"upstream_stream_incomplete"`)
}
```

- [ ] **Step 2: Run the regression test and verify RED**

Run:

```powershell
go test ./relay/channel/openai -run TestOaiResponsesStreamHandlerEmitsFailureForEOFWithoutTerminalEvent -count=1
```

Expected: FAIL because the current handler closes the stream without `response.failed`.

- [ ] **Step 3: Add terminal-event non-duplication coverage**

Add table coverage for upstream `response.completed` and `response.failed` events. Each input must produce its original terminal event exactly once and no synthetic duplicate.

- [ ] **Step 4: Run the new tests before implementation**

Run:

```powershell
go test ./relay/channel/openai -run "TestOaiResponsesStreamHandler(EmitsFailureForEOFWithoutTerminalEvent|DoesNotDuplicateTerminalEvent)" -count=1
```

Expected: the EOF case fails; existing upstream terminal events remain observable.

### Task 2: Implement protocol-aware terminal failure without returning a relay error

**Files:**
- Modify: `relay/channel/openai/relay_responses.go`
- Modify: `relay/channel/openai/helper.go`
- Test: `relay/channel/openai/relay_responses_test.go`

- [ ] **Step 1: Return downstream write errors**

Change the helper to return the existing response writer error:

```go
func sendResponsesStreamData(c *gin.Context, streamResponse dto.ResponsesStreamResponse, data string) error {
	if data == "" {
		return nil
	}
	return helper.ResponseChunkData(c, streamResponse, data)
}
```

In `OaiResponsesStreamHandler`, stop the scanner when forwarding fails:

```go
if err := sendResponsesStreamData(c, streamResponse, data); err != nil {
	sr.Stop(err)
	return
}
```

- [ ] **Step 2: Track Responses terminal events**

Before invoking `StreamScannerHandler`, add `terminalReceived := false` and response metadata fields. In the callback, mark these types terminal:

```go
switch streamResponse.Type {
case "response.completed", "response.done", "response.incomplete", "response.failed", "response.error":
	terminalReceived = true
}
```

Capture only response ID and normalized client model from an event response. Do not retain request data.

- [ ] **Step 3: Emit one synthetic terminal failure after the scanner returns**

When no terminal event arrived and the downstream remains writable, marshal a `dto.ResponsesStreamResponse` using `common.Marshal`:

```go
failure := dto.ResponsesStreamResponse{
	Type: "response.failed",
	Response: &dto.OpenAIResponsesResponse{
		ID:     responseID,
		Object: "response",
		Status: []byte(`"failed"`),
		Model:  responseModel,
		Error: types.OpenAIError{
			Type:    "server_error",
			Code:    "upstream_stream_incomplete",
			Message: "Upstream stream ended before completion.",
		},
	},
}
```

Skip emission for `client_gone` and `handler_stop`. Record the incomplete condition in `info.StreamStatus`, but return the calculated usage and `nil` relay error.

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run:

```powershell
go test ./relay/channel/openai -run "TestOaiResponsesStreamHandler" -count=1
```

Expected: PASS, including usage and cached-token extraction.

- [ ] **Step 5: Commit the protocol patch**

```powershell
git add relay/channel/openai/relay_responses.go relay/channel/openai/helper.go relay/channel/openai/relay_responses_test.go
git commit -m "fix: report incomplete responses streams"
```

### Task 3: Propagate downstream cancellation into the upstream request

**Files:**
- Modify: `relay/channel/api_request.go`
- Modify: `relay/channel/api_request_test.go`
- Test: `relay/channel/api_request_test.go`

- [ ] **Step 1: Write the failing cancellation test**

Create an `httptest.Server` whose handler waits on `r.Context().Done()`. Start `DoRequest` with a Gin request context, wait until upstream starts, cancel the Gin request, and assert both `DoRequest` and the upstream handler observe cancellation within two seconds.

The test must use `require` and deterministic channels, with no sleeps.

- [ ] **Step 2: Run the cancellation test and verify RED**

Run:

```powershell
go test ./relay/channel -run TestDoRequestCancelsUpstreamWhenClientDisconnects -count=1
```

Expected: FAIL because `client.Do` currently receives a request with a background context.

- [ ] **Step 3: Bind the outgoing request context**

Immediately before `client.Do(req)` in `doRequest`, preserve all request fields while binding the incoming context:

```go
if c != nil && c.Request != nil {
	req = req.WithContext(c.Request.Context())
}
```

- [ ] **Step 4: Run the cancellation test and verify GREEN**

Run:

```powershell
go test ./relay/channel -run TestDoRequestCancelsUpstreamWhenClientDisconnects -count=1
```

Expected: PASS with one upstream request and prompt cancellation.

- [ ] **Step 5: Run stream and request regression packages**

```powershell
go test ./relay/helper ./relay/channel ./relay/channel/openai -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit cancellation propagation separately**

```powershell
git add relay/channel/api_request.go relay/channel/api_request_test.go
git commit -m "fix: cancel upstream requests with clients"
```

### Task 4: Verify billing boundaries and full backend compatibility

**Files:**
- Verify only

- [ ] **Step 1: Confirm the accepted-stream contract**

Re-run the EOF regression and confirm `relayErr` is nil. This is the controller boundary that avoids `processChannelError`, retry, and `Billing.Refund` for an already accepted HTTP 200 stream.

- [ ] **Step 2: Confirm no billing files changed**

```powershell
git diff 4d192bf88 --name-only | rg "^(service/(billing|quota|pre_consume)|controller/relay\.go|model/)"
```

Expected: no output.

- [ ] **Step 3: Run the complete Go test suite**

```powershell
go test ./... -count=1
```

Expected: PASS.
