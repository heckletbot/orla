// Package api provides the HTTP API for the serving layer daemon.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dorcha-inc/orla/internal/core"
	"github.com/dorcha-inc/orla/internal/model"
	"github.com/dorcha-inc/orla/internal/serving"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// AgenticServer is the HTTP API server for the daemon
type AgenticServer struct {
	layer      *serving.AgenticLayer
	httpServer *http.Server
	mux        *http.ServeMux
}

// NewAgenticServer creates a new daemon API server
func NewAgenticServer(layer *serving.AgenticLayer, listenAddress string) *AgenticServer {
	mux := http.NewServeMux()
	server := &AgenticServer{
		layer: layer,
		mux:   mux,
		httpServer: &http.Server{
			Addr:              listenAddress,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}

	server.registerRoutes()
	return server
}

func (s *AgenticServer) registerRoutes() {
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/execute", s.handleExecute)
}

// Start starts the HTTP server
func (s *AgenticServer) Start() error {
	zap.L().Info("Starting daemon API server",
		zap.String("address", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server
func (s *AgenticServer) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *AgenticServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	core.WriteJSONResponse(w, map[string]string{
		"status": "healthy",
	})
}

// ExecuteRequest is the request body for the execute endpoint.
type ExecuteRequest struct {
	Server    string          `json:"server"`
	Prompt    string          `json:"prompt,omitempty"`
	Messages  []model.Message `json:"messages,omitempty"`
	Tools     []*mcp.Tool     `json:"tools,omitempty"`
	MaxTokens *int            `json:"max_tokens,omitempty"`
	Stream    bool            `json:"stream,omitempty"`
}

// ExecuteResponse is the response body for the execute endpoint.
type ExecuteResponse struct {
	Success  bool            `json:"success"`
	Response *model.Response `json:"response,omitempty"`
	Error    string          `json:"error,omitempty"`
}

func (s *AgenticServer) handleExecute(w http.ResponseWriter, r *http.Request) {
	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	if req.Server == "" {
		http.Error(w, "server is required", http.StatusBadRequest)
		return
	}

	messages := req.Messages
	if req.Prompt != "" {
		messages = append(messages, model.Message{
			Role:    model.MessageRoleUser,
			Content: req.Prompt,
		})
	}

	maxTokens := 0
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	ctx := r.Context()
	response, err := s.layer.Execute(ctx, req.Server, messages, req.Tools, serving.ExecuteOptions{
		MaxTokens: maxTokens,
		Stream:    req.Stream,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		core.WriteJSONResponse(w, ExecuteResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	zap.L().Debug("Executed inference via API",
		zap.String("server", req.Server),
		zap.Int("response_length", len(response.Content)))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	core.WriteJSONResponse(w, ExecuteResponse{
		Success:  true,
		Response: response,
	})
}
