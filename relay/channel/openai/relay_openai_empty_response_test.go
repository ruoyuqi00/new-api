package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOAITextResponseHasSignalRejectsEmptyResponse(t *testing.T) {
	require.False(t, oaiTextResponseHasSignal(dto.OpenAITextResponse{}))
	require.False(t, oaiTextResponseHasSignal(dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{{}},
	}))
}

func TestOAITextResponseHasSignalAcceptsContentFinishReasonAndUsage(t *testing.T) {
	require.True(t, oaiTextResponseHasSignal(dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{{
			Message: dto.Message{Content: "ok"},
		}},
	}))
	require.True(t, oaiTextResponseHasSignal(dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{{FinishReason: "stop"}},
	}))
	require.True(t, oaiTextResponseHasSignal(dto.OpenAITextResponse{
		Usage: dto.Usage{PromptTokens: 1},
	}))
}

func TestOAIStreamChunkHasSignalRejectsEmptyChunk(t *testing.T) {
	require.False(t, oaiStreamChunkHasSignal(nil))
	require.False(t, oaiStreamChunkHasSignal(&dto.ChatCompletionsStreamResponse{}))
	require.False(t, oaiStreamChunkHasSignal(&dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{{}},
	}))
}

func TestOAIStreamChunkHasSignalAcceptsContentFinishReasonToolAndUsage(t *testing.T) {
	content := "ok"
	finishReason := "stop"
	require.True(t, oaiStreamChunkHasSignal(&dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: &content},
		}},
	}))
	require.True(t, oaiStreamChunkHasSignal(&dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			FinishReason: &finishReason,
		}},
	}))
	require.True(t, oaiStreamChunkHasSignal(&dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{{Function: dto.FunctionResponse{Name: "lookup"}}},
			},
		}},
	}))
	require.True(t, oaiStreamChunkHasSignal(&dto.ChatCompletionsStreamResponse{
		Usage: &dto.Usage{PromptTokens: 1},
	}))
}

func TestOAIImageResponseHasSignal(t *testing.T) {
	require.False(t, oaiImageResponseHasSignal(dto.ImageResponse{}))
	require.False(t, oaiImageResponseHasSignal(dto.ImageResponse{
		Data: []dto.ImageData{{}},
	}))
	require.True(t, oaiImageResponseHasSignal(dto.ImageResponse{
		Data: []dto.ImageData{{Url: "https://example.com/image.png"}},
	}))
	require.True(t, oaiImageResponseHasSignal(dto.ImageResponse{
		Data: []dto.ImageData{{B64Json: "iVBORw0KGgo="}},
	}))
}

func TestOpenaiImageHandlerRejectsEmptyImageResponseBeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []string{
		`{}`,
		`{"created":1780000000,"data":[]}`,
		`{"created":1780000000,"data":[{}]}`,
	}

	for _, body := range testCases {
		t.Run(body, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}

			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI}}
			usage, err := OpenaiImageHandler(ctx, info, resp)

			require.Nil(t, usage)
			require.NotNil(t, err)
			require.Equal(t, types.ErrorCodeEmptyResponse, err.GetErrorCode())
			require.Equal(t, http.StatusBadGateway, err.StatusCode)
			require.Equal(t, 200, recorder.Code)
			require.Empty(t, recorder.Body.String())
		})
	}
}

func TestOpenaiImageHandlerAcceptsImagePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"created":1780000000,"data":[{"url":"https://example.com/image.png"}],"usage":{"total_tokens":3,"prompt_tokens":3}}`)),
	}

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI}}
	usage, err := OpenaiImageHandler(ctx, info, resp)

	require.Nil(t, err)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), "https://example.com/image.png")
}

func TestOpenaiImageHandlerConvertsURLToB64WhenRequested(t *testing.T) {
	gin.SetMode(gin.TestMode)

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{
			0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
			0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
			0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
			0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
			0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
			0x54, 0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f,
			0x00, 0x05, 0xfe, 0x02, 0xfe, 0xdc, 0xcc, 0x59,
			0xe7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
			0x44, 0xae, 0x42, 0x60, 0x82,
		})
	}))
	defer imageServer.Close()
	parsedURL, err := url.Parse(imageServer.URL)
	require.NoError(t, err)
	fetchSetting := system_setting.GetFetchSetting()
	oldPorts := append([]string(nil), fetchSetting.AllowedPorts...)
	oldAllowPrivateIP := fetchSetting.AllowPrivateIp
	oldMaxFileDownloadMB := constant.MaxFileDownloadMB
	fetchSetting.AllowedPorts = append(fetchSetting.AllowedPorts, parsedURL.Port())
	fetchSetting.AllowPrivateIp = true
	constant.MaxFileDownloadMB = 1
	service.InitHttpClient()
	t.Cleanup(func() {
		fetchSetting.AllowedPorts = oldPorts
		fetchSetting.AllowPrivateIp = oldAllowPrivateIP
		constant.MaxFileDownloadMB = oldMaxFileDownloadMB
		service.InitHttpClient()
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"created":1780000000,"data":[{"url":"` + imageServer.URL + `/image.png"}],"usage":{"total_tokens":3,"prompt_tokens":3}}`)),
	}

	info := &relaycommon.RelayInfo{
		Request:     &dto.ImageRequest{ResponseFormat: "b64_json"},
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
	}
	usage, err := OpenaiImageHandler(ctx, info, resp)

	require.Nil(t, err)
	require.NotNil(t, usage)
	require.Contains(t, recorder.Body.String(), `"b64_json"`)
	require.NotContains(t, recorder.Body.String(), imageServer.URL)
}
