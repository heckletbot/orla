package orla

import (
	"context"
	"fmt"
)

// Agent represents a single agent profile including the backend and inference options.
// Use it for execute calls and pass the prompt per call to Execute or ExecuteStream.
// Note that this is safe for concurrent use i.e. multiple threads can use the same Agent
// instance to execute calls.
type Agent struct {
	Client *OrlaClient
	Backend *LLMBackend
	// MaxTokens is optional; nil means backend default.
	MaxTokens *int
	// Temperature is optional; nil means backend default.
	Temperature *float64
	// TopP is optional; nil means backend default.
	TopP *float64
}

// NewAgent returns an agent that uses the given client and backend.
func NewAgent(client *OrlaClient, backend *LLMBackend) *Agent {
	return &Agent{Client: client, Backend: backend}
}

// SetMaxTokens sets the maximum tokens for execute calls (nil means backend default).
func (a *Agent) SetMaxTokens(n int) {
	a.MaxTokens = &n
}

// SetTemperature sets the sampling temperature for execute calls (nil means backend default).
func (a *Agent) SetTemperature(f float64) {
	a.Temperature = &f
}

// SetTopP sets the nucleus sampling top_p for execute calls (nil means backend default).
func (a *Agent) SetTopP(f float64) {
	a.TopP = &f
}

func (a *Agent) req(prompt string) *ExecuteRequest {
	r := &ExecuteRequest{Backend: a.Backend.Name, Prompt: prompt}
	r.MaxTokens = a.MaxTokens
	r.Temperature = a.Temperature
	r.TopP = a.TopP
	return r
}

// Execute runs a single non-streaming inference with the given prompt.
func (a *Agent) Execute(ctx context.Context, prompt string) (*InferenceResponse, error) {
	return a.Client.Execute(ctx, a.req(prompt))
}

// ExecuteStream runs inference with streaming; returns a channel of events (content, thinking, tool_call, done).
func (a *Agent) ExecuteStream(ctx context.Context, prompt string) (<-chan StreamEvent, error) {
	return a.Client.ExecuteStream(ctx, a.req(prompt))
}

// StreamHandler is an optional callback invoked for each stream event (e.g. to print tokens).
// ConsumeStream always accumulates and returns the full InferenceResponse; the handler is for side effects only.
type StreamHandler func(event StreamEvent) error

// ConsumeStream reads the stream until "done", accumulates content/thinking/metrics, and returns the result.
// If streamHandler is non-nil, it is called for each event before processing (e.g. to print content as it arrives).
func (a *Agent) ConsumeStream(ctx context.Context, stream <-chan StreamEvent, streamHandler StreamHandler) (*InferenceResponse, error) {
	response := &InferenceResponse{
		Content:     "",
		Thinking:    "",
		ToolCalls:   []any{},
		ToolResults: []any{},
		Metrics:     &InferenceResponseMetrics{},
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event, ok := <-stream:
			if !ok {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, fmt.Errorf("stream closed without a final response")
			}
			if streamHandler != nil {
				if err := streamHandler(event); err != nil {
					return nil, fmt.Errorf("stream handler: %w", err)
				}
			}
			switch event.Type {
			case "content":
				response.Content += event.Content
			case "thinking":
				response.Thinking += event.Thinking
			case "tool_call":
				return nil, fmt.Errorf("tool calls not supported for now")
			case "done":
				if event.Response != nil && event.Response.Metrics != nil {
					response.Metrics.TTFTMs = event.Response.Metrics.TTFTMs
					response.Metrics.TPOTMs = event.Response.Metrics.TPOTMs
				}
				return response, nil
			default:
				// Ignore unknown event types for forward compatibility
			}
		}
	}
}
