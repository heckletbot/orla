// Package orla provides a public Go client library for Orla server.
// Tool support uses the Model Context Protocol (MCP) types from github.com/modelcontextprotocol/go-sdk/mcp.

package orla

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolSchema is a JSON-serializable object (e.g. for tool input/output).
type ToolSchema map[string]any

// ToolRunner runs a tool: input from the model, result back to the model.
// Return a ToolResult with OutputValues (and optionally Error/IsError for tool-level failures).
// ID and Name are filled in by the agent; the runner only sets OutputValues, Error, IsError.
// Returning a non-nil error is treated as IsError true with Error set to err.Error().
type ToolRunner func(ctx context.Context, input ToolSchema) (*ToolResult, error)

// Tool defines a single tool: name, description, schemas, and runner.
type Tool struct {
	Name         string
	Description  string
	InputSchema  ToolSchema
	OutputSchema ToolSchema
	Run          ToolRunner
}

// NewTool returns a Tool. run must be non-nil.
func NewTool(name, description string, inputSchema, outputSchema ToolSchema, run ToolRunner) (*Tool, error) {
	if run == nil {
		return nil, fmt.Errorf("tool runner cannot be nil")
	}
	return &Tool{
		Name:         name,
		Description:  description,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Run:          run,
	}, nil
}

// ToolRunnerFromSchema wraps a simple (ToolSchema, error) function as a ToolRunner.
// Use this when you don't need to return tool-level Error/IsError; a returned Go error becomes result.IsError.
func ToolRunnerFromSchema(fn func(ctx context.Context, input ToolSchema) (ToolSchema, error)) ToolRunner {
	return func(ctx context.Context, input ToolSchema) (*ToolResult, error) {
		out, err := fn(ctx, input)
		if err != nil {
			return &ToolResult{Error: err.Error(), IsError: true}, nil
		}
		if out == nil {
			out = ToolSchema{}
		}
		return &ToolResult{OutputValues: out}, nil
	}
}

// toMCP returns the MCP tool spec for the execute request.
func (t *Tool) toMCP() *mcp.Tool {
	return &mcp.Tool{
		Name:         t.Name,
		Description:  t.Description,
		InputSchema:  t.InputSchema,
		OutputSchema: t.OutputSchema,
	}
}

// ToolCall is one tool invocation from the agent.
type ToolCall struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	InputArguments ToolSchema `json:"input_arguments"`
}

// ToolResult is the result of running one tool call.
type ToolResult struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	OutputValues ToolSchema `json:"output_values"`
	Error        string     `json:"error,omitempty"`
	IsError      bool       `json:"is_error,omitempty"`
}

// toolCallWithID is a tool call with an ID.
// NOTE: this is the same as orla/internal/model/types.go:toolCallWithID.
// If updating this, update the other one as well.
type toolCallWithID struct {
	ID                string `json:"id"`
	McpCallToolParams mcp.CallToolParams
}

func toolCallWithIDFromJSON(data []byte) (*toolCallWithID, error) {
	var tc toolCallWithID
	if err := json.Unmarshal(data, &tc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ToolCallWithID: %w", err)
	}
	return &tc, nil
}

func (tc *toolCallWithID) toToolCall() (*ToolCall, error) {
	args, ok := tc.McpCallToolParams.Arguments.(ToolSchema)
	if !ok {
		return nil, fmt.Errorf("failed to convert arguments to ToolSchema: %v", tc.McpCallToolParams.Arguments)
	}
	return &ToolCall{
		ID:             tc.ID,
		Name:           tc.McpCallToolParams.Name,
		InputArguments: args,
	}, nil
}

// NewToolCallFromRawToolCall converts a raw tool call from an InferenceResponse to a ToolCall.
// data is an element of the array InferenceResponse.ToolCalls.
func NewToolCallFromRawToolCall(rawToolCall RawToolCall) (*ToolCall, error) {
	if len(rawToolCall) == 0 {
		return nil, fmt.Errorf("rawToolCall is empty")
	}

	tc, err := toolCallWithIDFromJSON(rawToolCall)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal rawToolCall: %w", err)
	}

	return tc.toToolCall()
}

// ToMessage returns a tool-result message to append to the conversation.
func (r ToolResult) ToMessage() (*Message, error) {
	message := &Message{
		Role:       "tool",
		ToolCallID: r.ID,
		ToolName:   r.Name,
	}

	if r.IsError {
		toolPrefix := "tool execution error"
		if r.Error != "" {
			toolPrefix += ": " + r.Error
		}
		message.Content = toolPrefix
		return message, nil
	}

	if len(r.OutputValues) == 0 {
		return nil, fmt.Errorf("output values are empty")
	}

	outputValues, err := json.Marshal(r.OutputValues)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal output values: %w", err)
	}

	message.Content = string(outputValues)
	return message, nil
}
