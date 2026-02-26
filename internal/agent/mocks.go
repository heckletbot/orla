package agent

import (
	"context"
	"errors"

	"github.com/dorcha-inc/orla/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mockProvider is a mock implementation of model.Provider for testing
// Supports both function-based (for flexibility) and field-based (for simplicity) approaches
type mockProvider struct {
	name            string
	chatFunc        func(ctx context.Context, messages []model.Message, tools []*mcp.Tool, opts model.InferenceOptions) (*model.Response, <-chan model.StreamEvent, error)
	ensureReadyFunc func(ctx context.Context) error
	// Field-based approach (used when chatFunc is nil)
	chatResponse     *model.Response
	chatStreamCh     <-chan model.StreamEvent
	chatError        error
	ensureReadyError error
}

func (m *mockProvider) Name() string {
	if m == nil {
		return ""
	}
	return m.name
}

func (m *mockProvider) Chat(ctx context.Context, messages []model.Message, tools []*mcp.Tool, opts model.InferenceOptions) (*model.Response, <-chan model.StreamEvent, error) {
	if m == nil {
		return nil, nil, errors.New("nil mock provider")
	}
	if m.chatFunc != nil {
		return m.chatFunc(ctx, messages, tools, opts)
	}
	// Fall back to field-based approach
	if m.chatResponse == nil && m.chatError == nil {
		// Default response when using function-based approach without setting fields
		return &model.Response{Content: "test response"}, nil, nil
	}
	return m.chatResponse, m.chatStreamCh, m.chatError
}

func (m *mockProvider) EnsureReady(ctx context.Context) error {
	if m == nil {
		return errors.New("nil mock provider")
	}
	if m.ensureReadyFunc != nil {
		return m.ensureReadyFunc(ctx)
	}
	return m.ensureReadyError
}
