package common

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestValidateMultipartDirectNormalizesImageField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.NewReader(`{"model":"wan2.7-i2v","prompt":"animate","image":" https://example.com/first.png "}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	info := &RelayInfo{
		TaskRelayInfo: &TaskRelayInfo{},
	}

	taskErr := ValidateMultipartDirect(context, info)

	require.Nil(t, taskErr)
	storedReq, err := GetTaskRequest(context)
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com/first.png"}, storedReq.Images)
	require.Equal(t, constant.TaskActionGenerate, info.Action)
}

func TestTaskDurationBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newJSONContext := func(body string) (*gin.Context, *RelayInfo) {
		request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = request
		return context, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}
	}

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "duration above max is rejected",
			body:    fmt.Sprintf(`{"model":"sora-2","prompt":"a cat","duration":%d}`, MaxTaskDurationSeconds+1),
			wantErr: true,
		},
		{
			name:    "seconds string above max is rejected",
			body:    fmt.Sprintf(`{"model":"sora-2","prompt":"a cat","seconds":"%d"}`, MaxTaskDurationSeconds+1),
			wantErr: true,
		},
		{
			name:    "negative duration is rejected",
			body:    `{"model":"sora-2","prompt":"a cat","duration":-8}`,
			wantErr: true,
		},
		{
			name:    "non numeric seconds is rejected",
			body:    `{"model":"sora-2","prompt":"a cat","seconds":"abc"}`,
			wantErr: true,
		},
		{
			name:    "fractional metadata durationSeconds is rejected",
			body:    `{"model":"veo-3.1-generate-preview","prompt":"a cat","metadata":{"durationSeconds":8.5}}`,
			wantErr: true,
		},
		{
			name:    "metadata durationSeconds above max is rejected",
			body:    fmt.Sprintf(`{"model":"veo-3.1-generate-preview","prompt":"a cat","metadata":{"durationSeconds":%d}}`, MaxTaskDurationSeconds+1),
			wantErr: true,
		},
		{
			name:    "metadata parameters duration above max is rejected",
			body:    fmt.Sprintf(`{"model":"wan2.7-i2v","prompt":"a cat","image":"https://example.com/a.png","metadata":{"parameters":{"duration":%d}}}`, MaxTaskDurationSeconds+1),
			wantErr: true,
		},
		{
			name: "normal seconds is accepted",
			body: `{"model":"sora-2","prompt":"a cat","seconds":"8"}`,
		},
		{
			name: "zero duration keeps provider default behavior",
			body: `{"model":"sora-2","prompt":"a cat","duration":0}`,
		},
		{
			name: "max metadata durationSeconds is accepted",
			body: fmt.Sprintf(`{"model":"veo-3.1-generate-preview","prompt":"a cat","metadata":{"durationSeconds":%d}}`, MaxTaskDurationSeconds),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (multipart direct)", func(t *testing.T) {
			context, info := newJSONContext(tt.body)
			taskErr := ValidateMultipartDirect(context, info)
			requireTaskDurationResult(t, taskErr, tt.wantErr)
		})
		t.Run(tt.name+" (basic task request)", func(t *testing.T) {
			context, info := newJSONContext(tt.body)
			taskErr := ValidateBasicTaskRequest(context, info, constant.TaskActionGenerate)
			requireTaskDurationResult(t, taskErr, tt.wantErr)
		})
	}
}

func TestMultipartTaskDurationBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newMultipartContext := func(t *testing.T, seconds string) (*gin.Context, *RelayInfo) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "viduq1"))
		require.NoError(t, writer.WriteField("prompt", "a cat"))
		require.NoError(t, writer.WriteField("seconds", seconds))
		require.NoError(t, writer.Close())

		request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = request
		storage, err := appcommon.GetBodyStorage(context)
		require.NoError(t, err)
		context.Request.Body = io.NopCloser(storage)
		return context, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}
	}

	tests := []struct {
		name    string
		seconds string
		wantErr bool
	}{
		{
			name:    "non numeric seconds is rejected",
			seconds: "abc",
			wantErr: true,
		},
		{
			name:    "seconds above max is rejected",
			seconds: fmt.Sprintf("%d", MaxTaskDurationSeconds+1),
			wantErr: true,
		},
		{
			name:    "normal seconds is accepted",
			seconds: "8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context, info := newMultipartContext(t, tt.seconds)
			taskErr := ValidateBasicTaskRequest(context, info, constant.TaskActionGenerate)
			requireTaskDurationResult(t, taskErr, tt.wantErr)
		})
	}
}

func requireTaskDurationResult(t *testing.T, taskErr *dto.TaskError, wantErr bool) {
	t.Helper()
	if wantErr {
		require.NotNil(t, taskErr)
		require.Equal(t, "invalid_seconds", taskErr.Code)
		return
	}
	require.Nil(t, taskErr)
}
