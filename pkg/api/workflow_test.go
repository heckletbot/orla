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

func TestWorkflow_StageDAG(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ExecuteRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		content := req.Prompt
		if len(req.Messages) > 0 {
			content = req.Messages[len(req.Messages)-1].Content
		}
		encodeExecuteResponse(w, ExecuteResponse{
			Success:  true,
			Response: &InferenceResponse{Content: content},
		})
	}))
	defer server.Close()

	client := NewOrlaClient(server.URL)
	backend := &LLMBackend{Name: "b", Endpoint: server.URL, Type: "openai", ModelID: "openai:test"}

	s1 := NewStage("classify", backend)
	s1.Prompt = "classify task"

	s2 := NewStage("generate", backend)
	s2.Prompt = "generate code"

	wf := NewWorkflow(client)
	require.NoError(t, wf.AddStage(s1))
	require.NoError(t, wf.AddStage(s2))
	require.NoError(t, wf.AddDependency(s2.ID, s1.ID))

	results, err := wf.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.NotNil(t, results[s1.ID])
	assert.NotNil(t, results[s2.ID])
}

func TestWorkflow_StageCycle(t *testing.T) {
	client := NewOrlaClient("http://example.com")
	backend := &LLMBackend{Name: "b", Endpoint: "http://x", Type: "openai", ModelID: "openai:test"}

	s1 := NewStage("s1", backend)
	s1.Prompt = "p"

	s2 := NewStage("s2", backend)
	s2.Prompt = "p"

	wf := NewWorkflow(client)
	require.NoError(t, wf.AddStage(s1))
	require.NoError(t, wf.AddStage(s2))
	require.NoError(t, wf.AddDependency(s1.ID, s2.ID))
	require.NoError(t, wf.AddDependency(s2.ID, s1.ID))

	_, err := wf.Execute(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestWorkflow_AddStage_and_AddDependency(t *testing.T) {
	client := NewOrlaClient("http://localhost:8081")
	backend := &LLMBackend{Name: "b", Endpoint: "http://vllm:8000/v1", Type: "openai", ModelID: "m"}
	wf := NewWorkflow(client)

	s1 := NewStage("s1", backend)
	s2 := NewStage("s2", backend)

	require.NoError(t, wf.AddStage(s1))
	require.NoError(t, wf.AddStage(s2))
	require.NoError(t, wf.AddDependency(s2.ID, s1.ID))

	assert.Len(t, wf.Stages(), 2)
	assert.Same(t, client, s1.Client)
	assert.Same(t, client, s2.Client)
}

func TestWorkflow_AddStage_duplicateReturnsError(t *testing.T) {
	client := NewOrlaClient("http://localhost:8081")
	backend := &LLMBackend{Name: "b", Endpoint: "http://vllm:8000/v1", Type: "openai", ModelID: "m"}
	wf := NewWorkflow(client)
	s := NewStage("s", backend)
	require.NoError(t, wf.AddStage(s))
	err := wf.AddStage(s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
