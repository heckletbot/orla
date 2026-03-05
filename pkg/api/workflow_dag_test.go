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

func TestWorkflow_ExecuteDAG_SingleShotLinear(t *testing.T) {
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

	wf := NewWorkflow(client)

	s1 := NewStage("step1", backend)
	s1.Prompt = "first"
	require.NoError(t, wf.AddStage(s1))

	s2 := NewStage("step2", backend)
	s2.PromptBuilder = func(results map[string]*StageResult) (string, error) {
		return results[s1.ID].Response.Content + "+second", nil
	}
	require.NoError(t, wf.AddStage(s2))
	require.NoError(t, wf.AddDependency(s2.ID, s1.ID))

	results, err := wf.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "first", results[s1.ID].Response.Content)
	assert.Equal(t, "first+second", results[s2.ID].Response.Content)
}

func TestWorkflow_ExecuteDAG_FanOut(t *testing.T) {
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

	wf := NewWorkflow(client)

	root := NewStage("root", backend)
	root.Prompt = "root"
	require.NoError(t, wf.AddStage(root))

	branchA := NewStage("branchA", backend)
	branchA.Prompt = "A"
	require.NoError(t, wf.AddStage(branchA))
	require.NoError(t, wf.AddDependency(branchA.ID, root.ID))

	branchB := NewStage("branchB", backend)
	branchB.Prompt = "B"
	require.NoError(t, wf.AddStage(branchB))
	require.NoError(t, wf.AddDependency(branchB.ID, root.ID))

	results, err := wf.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "root", results[root.ID].Response.Content)
	assert.Equal(t, "A", results[branchA.ID].Response.Content)
	assert.Equal(t, "B", results[branchB.ID].Response.Content)
}

func TestWorkflow_ExecuteDAG_NoStagesReturnsEmpty(t *testing.T) {
	wf := NewWorkflow(NewOrlaClient("http://x"))
	results, err := wf.Execute(context.Background())
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestWorkflow_ExecuteDAG_AgentLoopMode(t *testing.T) {
	executeCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/execute" {
			executeCount++
		}
		encodeExecuteResponse(w, ExecuteResponse{
			Success:  true,
			Response: &InferenceResponse{Content: "done"},
		})
	}))
	defer server.Close()

	client := NewOrlaClient(server.URL)
	backend := &LLMBackend{Name: "b", Endpoint: server.URL, Type: "openai", ModelID: "openai:test"}

	wf := NewWorkflow(client)

	s := NewStage("loop", backend)
	s.Prompt = "do something"
	s.ExecutionMode = ExecutionModeAgentLoop
	s.MaxTurns = 3
	require.NoError(t, wf.AddStage(s))

	results, err := wf.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, results[s.ID])
	assert.Equal(t, "done", results[s.ID].Response.Content)
	assert.True(t, len(results[s.ID].Messages) > 0)
	assert.Equal(t, 1, executeCount)
}
