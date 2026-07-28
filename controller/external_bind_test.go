package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEmailBindRequiresDashboardSession(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/oauth/email/bind",
		strings.NewReader(`{"email":"attacker@example.test","code":"123456"}`),
	)
	context.Set("id", 1) // Simulates a PAT-authenticated UserAuth request.

	EmailBind(context)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestWeChatBindRequiresDashboardSession(t *testing.T) {
	previousEnabled := common.WeChatAuthEnabled
	t.Cleanup(func() { common.WeChatAuthEnabled = previousEnabled })

	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			common.WeChatAuthEnabled = enabled
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(
				http.MethodPost,
				"/api/oauth/wechat/bind",
				strings.NewReader(`{"code":"attacker-code"}`),
			)
			context.Set("id", 1) // Simulates a PAT-authenticated UserAuth request.

			WeChatBind(context)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}
