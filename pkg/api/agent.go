package orla

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Agent represents a single agent profile including the backend and inference options.
// Use it for execute calls and pass the prompt per call to Execute or ExecuteStream.
// Add tools with AddTool so the model can call them when using ExecuteWithMessages
// or ExecuteStreamWithMessages. Note that this is safe for concurrent use i.e.
// multiple threads can use the same Agent instance to execute calls.
type Agent struct {
	Client  *OrlaClient
	Backend *LLMBackend
	// MaxTokens is optional; nil means backend default.
	MaxTokens *int
	// Temperature is optional; nil means backend default.
	Temperature *float64
	// TopP is optional; nil means backend default.
	TopP *float64
	// Tools are the tools attached to this agent.
	Tools map[string]*Tool
}

// NewAgent returns an agent that uses the given client and backend.
func NewAgent(client *OrlaClient, backend *LLMBackend) *Agent {
	tools := make(map[string]*Tool)
	return &Agent{Client: client, Backend: backend, Tools: tools}
}

// SetMaxTokens sets the maximum tokens for execute calls (nil means backend default).
func (a *Agent) SetMaxTokens(n int) { a.MaxTokens = &n }

// SetTemperature sets the sampling temperature for execute calls (nil means backend default).
func (a *Agent) SetTemperature(f float64) { a.Temperature = &f }

// SetTopP sets the nucleus sampling top_p for execute calls (nil means backend default).
func (a *Agent) SetTopP(f float64) { a.TopP = &f }

// AddTool adds a tool to this agent. The tool spec is sent to the model via the
// configured LLM backend. Run is invoked locally when the model returns a tool call.
func (a *Agent) AddTool(t *Tool) error {
	if t == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	a.Tools[t.Name] = t
	return nil
}

// req builds a request with a prompt and optional inference options.
func (a *Agent) req(prompt string) *ExecuteRequest {
	r := &ExecuteRequest{Backend: a.Backend.Name, Prompt: prompt}
	r.MaxTokens = a.MaxTokens
	r.Temperature = a.Temperature
	r.TopP = a.TopP
	return r
}

// reqWithMessages builds a request with existing messages and tools, for agent loops.
func (a *Agent) reqWithMessages(messages []Message) *ExecuteRequest {
	r := &ExecuteRequest{Backend: a.Backend.Name, Messages: messages}
	r.MaxTokens = a.MaxTokens
	r.Temperature = a.Temperature
	r.TopP = a.TopP

	if len(a.Tools) > 0 {
		r.Tools = a.toolsToMCP()
	}

	return r
}

func (a *Agent) toolsToMCP() []*mcp.Tool {
	out := make([]*mcp.Tool, 0, len(a.Tools))
	for _, t := range a.Tools {
		out = append(out, t.toMCP())
	}
	return out
}

// Execute runs a single non-streaming inference with the given prompt.
func (a *Agent) Execute(ctx context.Context, prompt string) (*InferenceResponse, error) {
	return a.Client.Execute(ctx, a.req(prompt))
}

// ExecuteStream runs inference with streaming; returns a channel of events (content, thinking, tool_call, done).
func (a *Agent) ExecuteStream(ctx context.Context, prompt string) (<-chan StreamEvent, error) {
	return a.Client.ExecuteStream(ctx, a.req(prompt))
}

// ExecuteWithMessages runs a single non-streaming inference with the given message list and any tools attached to the agent.
// Use this for agent loops: append assistant and tool result messages, then call again until the model returns no tool calls.
func (a *Agent) ExecuteWithMessages(ctx context.Context, messages []Message) (*InferenceResponse, error) {
	return a.Client.Execute(ctx, a.reqWithMessages(messages))
}

// ExecuteStreamWithMessages runs streaming inference with the given message list and any tools attached to the agent.
func (a *Agent) ExecuteStreamWithMessages(ctx context.Context, messages []Message) (<-chan StreamEvent, error) {
	return a.Client.ExecuteStream(ctx, a.reqWithMessages(messages))
}

// StreamHandler is an optional callback invoked for each stream event (e.g. to print tokens).
// ConsumeStream always accumulates and returns the full InferenceResponse; the handler is for side effects only.
type StreamHandler func(event StreamEvent) error

// ConsumeStream reads the stream until "done", accumulates content/thinking/metrics, and returns the result.
// If streamHandler is non-nil, it is called for each event before processing (e.g. to print content as it arrives).
func (a *Agent) ConsumeStream(ctx context.Context, stream <-chan StreamEvent, handler StreamHandler) (*InferenceResponse, error) {
	response := &InferenceResponse{Metrics: &InferenceResponseMetrics{}}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event, ok := <-stream:
			if !ok {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, fmt.Errorf("stream closed without done")
			}
			if handler != nil {
				if err := handler(event); err != nil {
					return nil, err
				}
			}
			switch event.Type {
			case "content":
				response.Content += event.Content
			case "thinking":
				response.Thinking += event.Thinking
			case "tool_call":
				// Streaming tool_call deltas are for display; final ToolCalls come in "done"
			case "done":
				if event.Response != nil {
					response.Content = event.Response.Content
					response.Thinking = event.Response.Thinking
					response.ToolCalls = event.Response.ToolCalls
					if event.Response.Metrics != nil {
						response.Metrics = event.Response.Metrics
					}
				}
				return response, nil
			}
		}
	}
}

func (a *Agent) RunToolCall(ctx context.Context, toolCall *ToolCall) (*ToolResult, error) {
	if toolCall == nil {
		return nil, fmt.Errorf("tool call cannot be nil")
	}

	tool, ok := a.Tools[toolCall.Name]
	if !ok {
		return nil, fmt.Errorf("unknown tool %q", toolCall.Name)
	}

	toolResult, err := tool.Run(ctx, toolCall.InputArguments)

	if err != nil {
		return nil, fmt.Errorf("failed to run tool call: %w", err)
	}

	if toolResult == nil {
		return nil, fmt.Errorf("tool result is nil")
	}

	toolResult.ID = toolCall.ID
	toolResult.Name = toolCall.Name

	return toolResult, nil
}

// RunToolCallsInResponseAndGetToolResults parses the response's tool calls, runs each tool by name, and returns results.
func (a *Agent) RunToolCallsInResponseAndGetToolResults(ctx context.Context, response *InferenceResponse) ([]*ToolResult, error) {
	toolResults := make([]*ToolResult, 0, len(response.ToolCalls))

	for _, call := range response.ToolCalls {
		toolCall, err := NewToolCallFromRawToolCall(call)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tool call: %w", err)
		}

		toolResult, err := a.RunToolCall(ctx, toolCall)
		if err != nil {
			return nil, fmt.Errorf("failed to run tool call: %w", err)
		}
		toolResults = append(toolResults, toolResult)
	}
	return toolResults, nil
}

// RunToolCallsInResponse runs the tool calls in the response and returns the tool result messages.
func (a *Agent) RunToolCallsInResponse(ctx context.Context, response *InferenceResponse) ([]*Message, error) {
	toolResults, err := a.RunToolCallsInResponseAndGetToolResults(ctx, response)
	if err != nil {
		return nil, fmt.Errorf("failed to run tool calls: %w", err)
	}

	toolMessages := make([]*Message, 0, len(toolResults))
	for _, toolResult := range toolResults {
		toolMessage, err := toolResult.ToMessage()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool result to message: %w", err)
		}
		toolMessages = append(toolMessages, toolMessage)
	}

	return toolMessages, nil
}
