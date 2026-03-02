package orla

import (
	"context"
	"fmt"
	"maps"
	"sync"
)

// WorkflowPromptBuilder builds a node prompt using already-completed dependency results.
type WorkflowPromptBuilder func(results map[string]*InferenceResponse) (string, error)

// WorkflowNode defines one executable stage in a workflow graph.
type WorkflowNode struct {
	ID            string
	Stage         *AgentStage
	DependsOn     []string
	Prompt        string
	PromptBuilder WorkflowPromptBuilder
}

// Workflow is a lightweight DAG of executable agent stages.
type Workflow struct {
	nodes map[string]*WorkflowNode
}

// NewWorkflow creates an empty workflow.
func NewWorkflow() *Workflow {
	return &Workflow{nodes: make(map[string]*WorkflowNode)}
}

// AddNode adds a node to the workflow.
func (w *Workflow) AddNode(node *WorkflowNode) error {
	if node == nil {
		return fmt.Errorf("workflow node cannot be nil")
	}
	if node.ID == "" {
		return fmt.Errorf("workflow node id is required")
	}
	if node.Stage == nil {
		return fmt.Errorf("workflow node %q stage is required", node.ID)
	}
	if _, exists := w.nodes[node.ID]; exists {
		return fmt.Errorf("workflow node %q already exists", node.ID)
	}
	w.nodes[node.ID] = node
	return nil
}

// Nodes returns all nodes keyed by ID.
func (w *Workflow) Nodes() map[string]*WorkflowNode {
	out := make(map[string]*WorkflowNode, len(w.nodes))
	maps.Copy(out, w.nodes)
	return out
}

// ExecuteWorkflow executes the workflow DAG with dependency-aware scheduling.
// Independent nodes execute concurrently; fan-out and fan-in are naturally supported via DependsOn.
func (c *OrlaClient) ExecuteWorkflow(ctx context.Context, workflow *Workflow) (map[string]*InferenceResponse, error) {
	if workflow == nil {
		return nil, fmt.Errorf("workflow cannot be nil")
	}
	if len(workflow.nodes) == 0 {
		return map[string]*InferenceResponse{}, nil
	}

	dependents := make(map[string][]string, len(workflow.nodes))
	remainingDeps := make(map[string]int, len(workflow.nodes))
	for id, node := range workflow.nodes {
		for _, depID := range node.DependsOn {
			if _, ok := workflow.nodes[depID]; !ok {
				return nil, fmt.Errorf("workflow node %q depends on unknown node %q", id, depID)
			}
			dependents[depID] = append(dependents[depID], id)
		}
		remainingDeps[id] = len(node.DependsOn)
	}

	results := make(map[string]*InferenceResponse, len(workflow.nodes))
	var resultsMu sync.RWMutex
	var remainingMu sync.Mutex

	type nodeOutcome struct {
		id        string
		executed  bool
		err       error
		unblocked []string
	}

	outcomeCh := make(chan nodeOutcome, len(workflow.nodes))
	readyCh := make(chan string, len(workflow.nodes))

	startNode := func(nodeID string) {
		go func() {
			node := workflow.nodes[nodeID]
			agent := NewAgent(c)
			agent.SetStage(node.Stage)

			resultsMu.RLock()
			depSnapshot := make(map[string]*InferenceResponse)
			maps.Copy(depSnapshot, results)
			resultsMu.RUnlock()

			prompt := node.Prompt
			if node.PromptBuilder != nil {
				builtPrompt, err := node.PromptBuilder(depSnapshot)
				if err != nil {
					outcomeCh <- nodeOutcome{id: nodeID, err: fmt.Errorf("node %q prompt builder failed: %w", nodeID, err)}
					return
				}
				prompt = builtPrompt
			}
			if prompt == "" {
				outcomeCh <- nodeOutcome{id: nodeID, err: fmt.Errorf("node %q prompt is empty", nodeID)}
				return
			}

			resp, err := agent.Execute(ctx, prompt)
			if err != nil {
				outcomeCh <- nodeOutcome{id: nodeID, err: fmt.Errorf("node %q execution failed: %w", nodeID, err)}
				return
			}

			resultsMu.Lock()
			results[nodeID] = resp
			resultsMu.Unlock()

			remainingMu.Lock()
			unblocked := make([]string, 0, len(dependents[nodeID]))
			for _, dep := range dependents[nodeID] {
				remainingDeps[dep]--
				if remainingDeps[dep] == 0 {
					unblocked = append(unblocked, dep)
				}
			}
			remainingMu.Unlock()

			outcomeCh <- nodeOutcome{id: nodeID, executed: true, unblocked: unblocked}
		}()
	}

	remainingMu.Lock()
	for id, deps := range remainingDeps {
		if deps == 0 {
			readyCh <- id
		}
	}
	remainingMu.Unlock()

	dispatched := 0
	completed := 0
	for {
		for {
			select {
			case nodeID := <-readyCh:
				startNode(nodeID)
				dispatched++
			default:
				goto waitOutcome
			}
		}

	waitOutcome:
		if completed == len(workflow.nodes) {
			break
		}
		// No running tasks and no ready tasks implies a dependency cycle.
		if dispatched == completed {
			select {
			case nodeID := <-readyCh:
				startNode(nodeID)
				dispatched++
				continue
			default:
				return nil, fmt.Errorf("workflow has at least one cycle; completed %d/%d nodes", completed, len(workflow.nodes))
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case outcome := <-outcomeCh:
			if outcome.err != nil {
				return nil, outcome.err
			}
			if outcome.executed {
				completed++
				for _, next := range outcome.unblocked {
					readyCh <- next
				}
			}
		}
	}

	if dispatched != len(workflow.nodes) {
		return nil, fmt.Errorf("workflow has at least one cycle; dispatched %d/%d nodes", dispatched, len(workflow.nodes))
	}
	return results, nil
}
