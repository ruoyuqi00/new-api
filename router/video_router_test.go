package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVideoRouterRegistersXAIImagineGenerationAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	SetVideoRouter(engine)
	routes := engine.Routes()
	registered := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		registered[route.Method+" "+route.Path] = struct{}{}
	}

	for _, route := range []string{
		http.MethodPost + " /v1/videos/generations",
		http.MethodGet + " /v1/videos/generations/:task_id",
	} {
		_, ok := registered[route]
		require.True(t, ok, "missing route %s", route)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/videos/generations",
		strings.NewReader(`{"model":"grok-imagine-video","prompt":"test"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
