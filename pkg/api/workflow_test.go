package orla

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteWorkflow_LinearAndFanIn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ExecuteRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		encodeExecuteResponse(w, ExecuteResponse{
			Success:  true,
			Response: &InferenceResponse{Content: req.Prompt},
		})
	}))
	defer server.Close()

	client := NewOrlaClient(server.URL)
	backend := &LLMBackend{Name: "b", Endpoint: server.URL, Type: "openai", ModelID: "openai:test"}
	stage := NewAgentStage("s", backend)

	wf := NewWorkflow()
	require.NoError(t, wf.AddNode(&WorkflowNode{ID: "summarize", Stage: stage, Prompt: "summary"}))
	require.NoError(t, wf.AddNode(&WorkflowNode{ID: "candidateA", Stage: stage, Prompt: "candidate A", DependsOn: []string{"summarize"}}))
	require.NoError(t, wf.AddNode(&WorkflowNode{ID: "candidateB", Stage: stage, Prompt: "candidate B", DependsOn: []string{"summarize"}}))
	require.NoError(t, wf.AddNode(&WorkflowNode{
		ID:        "aggregate",
		Stage:     stage,
		DependsOn: []string{"candidateA", "candidateB"},
		PromptBuilder: func(results map[string]*InferenceResponse) (string, error) {
			return results["candidateA"].Content + " + " + results["candidateB"].Content, nil
		},
	}))

	results, err := client.ExecuteWorkflow(context.Background(), wf)
	require.NoError(t, err)
	require.Len(t, results, 4)
	assert.Equal(t, "summary", results["summarize"].Content)
	assert.Equal(t, "candidate A", results["candidateA"].Content)
	assert.Equal(t, "candidate B", results["candidateB"].Content)
	assert.Equal(t, "candidate A + candidate B", results["aggregate"].Content)
}

func TestExecuteWorkflow_Cycle(t *testing.T) {
	client := NewOrlaClient("http://example.com")
	backend := &LLMBackend{Name: "b", Endpoint: "http://x", Type: "openai", ModelID: "openai:test"}
	stage := NewAgentStage("s", backend)

	wf := NewWorkflow()
	require.NoError(t, wf.AddNode(&WorkflowNode{ID: "a", Stage: stage, Prompt: "a", DependsOn: []string{"b"}}))
	require.NoError(t, wf.AddNode(&WorkflowNode{ID: "b", Stage: stage, Prompt: "b", DependsOn: []string{"a"}}))

	_, err := client.ExecuteWorkflow(context.Background(), wf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}
