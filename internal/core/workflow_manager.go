package core

import (
	"fmt"
	"sync"
)

// WorkflowManager is the single source of truth for workflow state.
// Both the memory manager and access control system reference it.
type WorkflowManager struct {
	mu        sync.Mutex
	workflows map[string]*WorkflowState
}

// NewWorkflowManager creates an empty workflow manager.
func NewWorkflowManager() *WorkflowManager {
	return &WorkflowManager{workflows: make(map[string]*WorkflowState)}
}

// Register initializes tracking for a workflow.
// This method is idempotent, meaning that calling it multiple times
// with the same workflowID will have no effect.
func (m *WorkflowManager) Register(workflowID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.workflows[workflowID]; !exists {
		m.workflows[workflowID] = NewWorkflowState(workflowID)
	}
}

// Deregister removes all state for a workflow.
func (m *WorkflowManager) Deregister(workflowID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.workflows, workflowID)
}

// Get returns the workflow state, or nil if not found.
// The caller must not hold the returned pointer across concurrent operations
// without the manager's lock. For most uses, call the manager's methods instead.
func (m *WorkflowManager) Get(workflowID string) *WorkflowState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.workflows[workflowID]
}

// ActiveWorkflowIDs returns the IDs of all active workflows.
func (m *WorkflowManager) ActiveWorkflowIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.workflows))
	for id := range m.workflows {
		ids = append(ids, id)
	}
	return ids
}

// RegisterEdges stores the DAG edges for a workflow.
// Each edge is [from_stageID, to_stageID]. The workflow is auto-registered
// if it doesn't exist yet.
func (m *WorkflowManager) RegisterEdges(workflowID string, edges [][2]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ws, ok := m.workflows[workflowID]
	if !ok {
		ws = NewWorkflowState(workflowID)
		m.workflows[workflowID] = ws
	}
	for _, e := range edges {
		ws.Edges[e[0]] = append(ws.Edges[e[0]], e[1])
	}
}

// RegisterDeclassifications records which labels each stage strips from propagation.
// The workflow is auto-registered if it doesn't exist yet.
// declassifications maps stageID -> list of labels that stage declassifies.
func (m *WorkflowManager) RegisterDeclassifications(workflowID string, declassifications map[string][]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ws, ok := m.workflows[workflowID]
	if !ok {
		ws = NewWorkflowState(workflowID)
		m.workflows[workflowID] = ws
	}
	for stageID, labels := range declassifications {
		set := ws.Declassifies[stageID]
		if set == nil {
			set = make(map[string]struct{})
			ws.Declassifies[stageID] = set
		}
		for _, l := range labels {
			set[l] = struct{}{}
		}
	}
}

// EffectiveLabels returns the data labels that apply to a stage: its own
// explicit labels merged with labels inherited from upstream stages in the DAG.
// Returns an error if the workflow is not registered.
func (m *WorkflowManager) EffectiveLabels(workflowID, stageID string, requestLabels []string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ws, ok := m.workflows[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow %q not registered", workflowID)
	}
	return ws.EffectiveLabels(stageID, requestLabels), nil
}
