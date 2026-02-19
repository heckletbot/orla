// Package orla provides a public Go client library for the Orla serving layer daemon.
//
// Example usage:
//
//	client := orla.NewClient("http://localhost:8081")
//	resp, err := client.Execute(ctx, &orla.ExecuteRequest{
//	    Server: "my-server",
//	    Prompt: "What is the weather in SF?",
//	})
package orla

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is the public API client for the Orla daemon
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new daemon API client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// Health checks the health of the daemon
func (c *Client) Health(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/health", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to check health: %w", err)
	}
	defer LogDeferredError(resp.Body.Close)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// ExecuteRequest represents a request to execute inference on a named server.
type ExecuteRequest struct {
	Server    string      `json:"server"`
	Prompt    string      `json:"prompt,omitempty"`
	Messages  []Message   `json:"messages,omitempty"`
	Tools     interface{} `json:"tools,omitempty"` // MCP tools ([]*mcp.Tool) or any JSON-serializable tool list
	MaxTokens int         `json:"max_tokens,omitempty"`
	Stream    bool        `json:"stream,omitempty"`
}

// ExecuteResponse represents the response from an execute call.
type ExecuteResponse struct {
	Success  bool          `json:"success"`
	Response *TaskResponse `json:"response,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// TaskResponse represents the response from inference.
type TaskResponse struct {
	Content     string               `json:"content"`
	Thinking    string               `json:"thinking,omitempty"`
	ToolCalls   []any                `json:"tool_calls,omitempty"`
	ToolResults []any                `json:"tool_results,omitempty"`
	Metrics     *TaskResponseMetrics `json:"metrics,omitempty"`
}

// TaskResponseMetrics holds timing metrics from streaming execution.
type TaskResponseMetrics struct {
	TTFTMs int64 `json:"ttft_ms,omitempty"`
	TPOTMs int64 `json:"tpot_ms,omitempty"`
}

// Execute runs inference on the named server via the daemon.
func (c *Client) Execute(ctx context.Context, req *ExecuteRequest) (*TaskResponse, error) {
	url := fmt.Sprintf("%s/api/v1/execute", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create execute request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request failed: %w", err)
	}
	defer LogDeferredError(httpResp.Body.Close)

	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return nil, fmt.Errorf("execute failed with status %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var execResp ExecuteResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&execResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !execResp.Success {
		return nil, fmt.Errorf("execution failed: %s", execResp.Error)
	}

	return execResp.Response, nil
}
