package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetModelRequestPreservesImageSelectionFields(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2-1k","size":"1k","aspect_ratio":"2:3","prompt":"portrait"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	request, shouldSelectChannel, err := getModelRequest(c)
	require.NoError(t, err)
	require.True(t, shouldSelectChannel)
	require.Equal(t, "1k", request.Size)
	require.Equal(t, "2:3", request.AspectRatio)
	require.Equal(t, &model.ImageSelectionRequirements{Size: "1k", AspectRatio: "2:3"}, imageSelectionRequirementsForRequest(c.Request.URL.Path, request))
}
