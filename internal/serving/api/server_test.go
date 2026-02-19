package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dorcha-inc/orla/internal/serving"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_HandleHealth(t *testing.T) {
	layer := serving.NewAgenticLayer()
	server := NewAgenticServer(layer, ":0")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	server.mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	var result map[string]string
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "healthy", result["status"])
}

func TestServer_HandleExecute_NoServer(t *testing.T) {
	layer := serving.NewAgenticLayer()
	server := NewAgenticServer(layer, ":0")

	reqBody := ExecuteRequest{
		Prompt: "test prompt",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/execute", bytes.NewReader(body))
	server.mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestServer_HandleExecute_InvalidJSON(t *testing.T) {
	layer := serving.NewAgenticLayer()
	server := NewAgenticServer(layer, ":0")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/execute", bytes.NewReader([]byte("invalid json")))
	server.mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestServer_HandleExecute_ServerNotFound(t *testing.T) {
	layer := serving.NewAgenticLayer()
	server := NewAgenticServer(layer, ":0")

	reqBody := ExecuteRequest{
		Server: "nonexistent",
		Prompt: "test prompt",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/execute", bytes.NewReader(body))
	server.mux.ServeHTTP(resp, req)

	require.Equal(t, http.StatusInternalServerError, resp.Code)
	var result ExecuteResponse
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "not found")
}
