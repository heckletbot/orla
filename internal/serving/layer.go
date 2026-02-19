// Package serving implements a minimal programmatic serving layer.
package serving

import (
	"context"
	"fmt"

	"github.com/dorcha-inc/orla/internal/core"
	"github.com/dorcha-inc/orla/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// AgenticLayer is the serving layer that manages LLM backends and executes inference.
type AgenticLayer struct {
	llmBackendManager *LLMBackendManager
}

// ExecuteOptions are additional options for executing inference.
type ExecuteOptions struct {
	MaxTokens int
	Stream    bool
}

// NewAgenticLayer creates a new serving layer.
func NewAgenticLayer() *AgenticLayer {
	return &AgenticLayer{
		llmBackendManager: NewLLMBackendManager(),
	}
}

// AddLLMBackend registers an LLM backend by name.
func (l *AgenticLayer) AddLLMBackend(name string, backend *core.LLMBackend, modelID string) {
	l.llmBackendManager.AddLLMBackend(name, backend, modelID)
}

// GetModelProvider returns the model provider for a named LLM backend.
func (l *AgenticLayer) GetModelProvider(ctx context.Context, backendName string) (model.Provider, error) {
	return l.llmBackendManager.GetModelProvider(ctx, backendName)
}

// Execute runs a single inference call against the named LLM backend.
// Streaming is not yet supported, so the layer always requests a full response.
func (l *AgenticLayer) Execute(ctx context.Context, serverName string, messages []model.Message, tools []*mcp.Tool, options ExecuteOptions) (*model.Response, error) {
	if options.Stream {
		return nil, fmt.Errorf("streaming is not yet supported")
	}

	provider, err := l.llmBackendManager.GetModelProvider(ctx, serverName)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider for server '%s': %w", serverName, err)
	}

	// Request non-streaming so we get a complete response; stream channel is not exposed by the layer yet.
	// TODO(jadidbourbaki): Add streaming support to the layer.
	response, _, err := provider.Chat(ctx, messages, tools, false, options.MaxTokens)
	if err != nil {
		return nil, fmt.Errorf("inference failed on server '%s': %w", serverName, err)
	}

	zap.L().Debug("Executed inference",
		zap.String("server", serverName),
		zap.Int("response_length", len(response.Content)))

	return response, nil
}

// GetLLMBackendHealth returns the health status of a named LLM backend.
func (l *AgenticLayer) GetLLMBackendHealth(ctx context.Context, serverName string) (HealthStatus, error) {
	return l.llmBackendManager.GetHealthStatus(ctx, serverName)
}

// ListLLMBackends returns all registered LLM backend names.
func (l *AgenticLayer) ListLLMBackends() []string {
	return l.llmBackendManager.ListLLMBackends()
}
