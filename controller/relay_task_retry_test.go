package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldRetryTaskRelayDoesNotRetryUnprocessableEntity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	retry := shouldRetryTaskRelay(c, 2360, &dto.TaskError{StatusCode: http.StatusUnprocessableEntity}, 2)

	require.False(t, retry)
}
