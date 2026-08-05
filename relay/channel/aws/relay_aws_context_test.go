package aws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	awsSDK "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAwsInvokeContextInheritsParentCancellation(t *testing.T) {
	originalRelayTimeout := common.RelayTimeout
	t.Cleanup(func() {
		common.RelayTimeout = originalRelayTimeout
	})

	for _, test := range []struct {
		name         string
		relayTimeout int
		wantDeadline bool
	}{
		{name: "without relay timeout", relayTimeout: 0, wantDeadline: false},
		{name: "with relay timeout", relayTimeout: 30, wantDeadline: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			common.RelayTimeout = test.relayTimeout
			parent, cancelParent := context.WithCancel(context.Background())
			invokeContext, cancelInvoke := newAwsInvokeContext(parent)
			t.Cleanup(cancelInvoke)

			_, hasDeadline := invokeContext.Deadline()
			assert.Equal(t, test.wantDeadline, hasDeadline)

			cancelParent()
			require.ErrorIs(t, invokeContext.Err(), context.Canceled)
		})
	}
}

func TestAwsHandlerSkipsRetryWhenRequestContextIsCanceled(t *testing.T) {
	requestContext, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()

	request := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(requestContext)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = request

	client := bedrockruntime.New(bedrockruntime.Options{
		Region:      "us-east-1",
		Credentials: awsSDK.NewCredentialsCache(credentials.NewStaticCredentialsProvider("test", "test", "")),
	})
	adaptor := &Adaptor{
		AwsClient: client,
		AwsReq: &bedrockruntime.InvokeModelInput{
			ModelId:     awsSDK.String("anthropic.claude-test"),
			Accept:      awsSDK.String("application/json"),
			ContentType: awsSDK.String("application/json"),
			Body:        []byte(`{}`),
		},
	}

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-test"}}
	apiErr, _ := awsHandler(c, info, adaptor)
	require.NotNil(t, apiErr)
	assert.True(t, types.IsSkipRetryError(apiErr))
}

func TestNewAwsInvokeErrorSkipsRetryWhenRequestDeadlineExpires(t *testing.T) {
	requestContext, cancelRequest := context.WithTimeout(context.Background(), 0)
	t.Cleanup(cancelRequest)
	require.ErrorIs(t, requestContext.Err(), context.DeadlineExceeded)

	apiErr := newAwsInvokeError(requestContext, context.DeadlineExceeded, "InvokeModel")
	assert.True(t, types.IsSkipRetryError(apiErr))

	relayTimeoutErr := newAwsInvokeError(context.Background(), context.DeadlineExceeded, "InvokeModel")
	assert.False(t, types.IsSkipRetryError(relayTimeoutErr))
}
