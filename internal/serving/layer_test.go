package serving

import (
	"context"
	"testing"

	"github.com/dorcha-inc/orla/internal/core"
	"github.com/dorcha-inc/orla/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLayer_NewLayer(t *testing.T) {
	layer := NewAgenticLayer()
	require.NotNil(t, layer)
	assert.Empty(t, layer.ListLLMBackends())
}

func TestLayer_AddServer(t *testing.T) {
	layer := NewAgenticLayer()
	layer.AddLLMBackend("test-server", &core.LLMBackend{
		Type:     core.LLMInferenceAPITypeOpenAI,
		Endpoint: "http://localhost:11434/v1",
	}, "openai:test-model")
	assert.Contains(t, layer.ListLLMBackends(), "test-server")
}

func TestLayer_Execute_WithMaxTokens(t *testing.T) {
	layer := NewAgenticLayer()
	layer.AddLLMBackend("test-server", &core.LLMBackend{
		Type:     core.LLMInferenceAPITypeOpenAI,
		Endpoint: "http://localhost:11434/v1",
	}, "openai:test-model")

	mock := model.NewMockProvider().WithContent("test response").Build()
	layer.llmBackendManager.mu.Lock()
	layer.llmBackendManager.providers["test-server"] = mock
	layer.llmBackendManager.mu.Unlock()

	response, err := layer.Execute(context.Background(), "test-server", "test", []model.Message{
		{Role: model.MessageRoleUser, Content: "test prompt"},
	}, nil, model.InferenceOptions{MaxTokens: core.IntPtr(42)})
	require.NoError(t, err)
	assert.Equal(t, "test response", response.Content)
	lastOpts := mock.LastInferenceOptions()
	require.NotNil(t, lastOpts)
	require.NotNil(t, lastOpts.MaxTokens)
	assert.Equal(t, 42, *lastOpts.MaxTokens)
}

func TestLayer_Execute_WithoutMaxTokens(t *testing.T) {
	layer := NewAgenticLayer()
	layer.AddLLMBackend("test-server", &core.LLMBackend{
		Type:     core.LLMInferenceAPITypeOpenAI,
		Endpoint: "http://localhost:11434/v1",
	}, "openai:test-model")

	mock := model.NewMockProvider().WithContent("test response").Build()
	layer.llmBackendManager.mu.Lock()
	layer.llmBackendManager.providers["test-server"] = mock
	layer.llmBackendManager.mu.Unlock()

	response, err := layer.Execute(context.Background(), "test-server", "test", []model.Message{
		{Role: model.MessageRoleUser, Content: "test prompt"},
	}, nil, model.InferenceOptions{})
	require.NoError(t, err)
	assert.Equal(t, "test response", response.Content)
	lastOpts := mock.LastInferenceOptions()
	require.NotNil(t, lastOpts)
	assert.Nil(t, lastOpts.MaxTokens)
}

func TestLayer_Execute_ServerNotFound(t *testing.T) {
	layer := NewAgenticLayer()
	_, err := layer.Execute(context.Background(), "nonexistent", "", nil, nil, model.InferenceOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLayer_Execute_RejectsStream(t *testing.T) {
	layer := NewAgenticLayer()
	layer.AddLLMBackend("test-server", &core.LLMBackend{
		Type:     core.LLMInferenceAPITypeOpenAI,
		Endpoint: "http://localhost:11434/v1",
	}, "openai:test-model")

	_, err := layer.Execute(context.Background(), "test-server", "test", []model.Message{
		{Role: model.MessageRoleUser, Content: "test"},
	}, nil, model.InferenceOptions{Stream: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ExecuteStream")
}

func TestLayer_ExecuteStream(t *testing.T) {
	layer := NewAgenticLayer()
	layer.AddLLMBackend("test-server", &core.LLMBackend{
		Type:     core.LLMInferenceAPITypeOpenAI,
		Endpoint: "http://localhost:11434/v1",
	}, "openai:test-model")

	mock := model.NewMockProvider().WithContent("test response").Build()
	layer.llmBackendManager.mu.Lock()
	layer.llmBackendManager.providers["test-server"] = mock
	layer.llmBackendManager.mu.Unlock()

	response, ch, err := layer.ExecuteStream(context.Background(), "test-server", "test", []model.Message{
		{Role: model.MessageRoleUser, Content: "test"},
	}, nil, model.InferenceOptions{Stream: true, MaxTokens: core.IntPtr(10)})
	require.NoError(t, err)
	assert.Equal(t, "test response", response.Content)
	if ch != nil {
		for range ch {
		}
	}
}
