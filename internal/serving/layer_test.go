package serving

import (
	"context"
	"testing"

	"github.com/dorcha-inc/orla/internal/core"
	"github.com/dorcha-inc/orla/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProvider struct {
	name          string
	lastMaxTokens *int
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Chat(_ context.Context, _ []model.Message, _ []*mcp.Tool, _ bool, maxTokens int) (*model.Response, <-chan model.StreamEvent, error) {
	if maxTokens != 0 {
		m.lastMaxTokens = &maxTokens
	} else {
		m.lastMaxTokens = nil
	}
	return &model.Response{
		Content: "test response",
	}, nil, nil
}

func (m *mockProvider) EnsureReady(_ context.Context) error {
	return nil
}

func TestLayer_NewLayer(t *testing.T) {
	layer := NewAgenticLayer()
	require.NotNil(t, layer)
	assert.Empty(t, layer.ListLLMBackends())
}

func TestLayer_AddServer(t *testing.T) {
	layer := NewAgenticLayer()
	layer.AddLLMBackend("test-server", &core.LLMBackend{
		Type:     core.LLMInferenceAPITypeOllama,
		Endpoint: "http://localhost:11434",
	}, "ollama:test-model")
	assert.Contains(t, layer.ListLLMBackends(), "test-server")
}

func TestLayer_Execute_WithMaxTokens(t *testing.T) {
	layer := NewAgenticLayer()
	layer.AddLLMBackend("test-server", &core.LLMBackend{
		Type:     core.LLMInferenceAPITypeOllama,
		Endpoint: "http://localhost:11434",
	}, "ollama:test-model")

	mock := &mockProvider{name: "mock"}
	layer.llmBackendManager.mu.Lock()
	layer.llmBackendManager.providers["test-server"] = mock
	layer.llmBackendManager.mu.Unlock()

	response, err := layer.Execute(context.Background(), "test-server", []model.Message{
		{Role: model.MessageRoleUser, Content: "test prompt"},
	}, nil, ExecuteOptions{MaxTokens: 42})
	require.NoError(t, err)
	assert.Equal(t, "test response", response.Content)
	assert.NotNil(t, mock.lastMaxTokens)
	assert.Equal(t, 42, *mock.lastMaxTokens)
}

func TestLayer_Execute_WithoutMaxTokens(t *testing.T) {
	layer := NewAgenticLayer()
	layer.AddLLMBackend("test-server", &core.LLMBackend{
		Type:     core.LLMInferenceAPITypeOllama,
		Endpoint: "http://localhost:11434",
	}, "ollama:test-model")

	mock := &mockProvider{name: "mock"}
	layer.llmBackendManager.mu.Lock()
	layer.llmBackendManager.providers["test-server"] = mock
	layer.llmBackendManager.mu.Unlock()

	response, err := layer.Execute(context.Background(), "test-server", []model.Message{
		{Role: model.MessageRoleUser, Content: "test prompt"},
	}, nil, ExecuteOptions{})
	require.NoError(t, err)
	assert.Equal(t, "test response", response.Content)
	assert.Nil(t, mock.lastMaxTokens)
}

func TestLayer_Execute_ServerNotFound(t *testing.T) {
	layer := NewAgenticLayer()
	_, err := layer.Execute(context.Background(), "nonexistent", nil, nil, ExecuteOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
