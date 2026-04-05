package memory

import (
	"sync"
	"time"

	"github.com/harvard-cns/orla/internal/core"
)

// Tracker maintains cache-specific and in-flight request state for active workflows.
// Workflow lifecycle is managed by core.WorkflowManager; the Tracker delegates
// to it for workflow registration and lookup.
type Tracker struct {
	mu       sync.Mutex
	wm       *core.WorkflowManager
	inflight map[string]map[string]*core.InflightRequest // backend -> requestID -> request
}

// NewTracker creates a new tracker backed by the given workflow manager.
func NewTracker(wm *core.WorkflowManager) *Tracker {
	return &Tracker{
		wm:       wm,
		inflight: make(map[string]map[string]*core.InflightRequest),
	}
}

// RegisterWorkflow initializes tracking for a new workflow execution.
func (t *Tracker) RegisterWorkflow(workflowID string) {
	t.wm.Register(workflowID)
}

// DeregisterWorkflow removes a workflow from tracking.
func (t *Tracker) DeregisterWorkflow(workflowID string) {
	t.wm.Deregister(workflowID)
}

// GetWorkflow returns a snapshot of a workflow's state, or nil if not found.
func (t *Tracker) GetWorkflow(workflowID string) *core.WorkflowState {
	return t.wm.Get(workflowID)
}

// ActiveWorkflowIDs returns the IDs of all active workflows.
func (t *Tracker) ActiveWorkflowIDs() []string {
	return t.wm.ActiveWorkflowIDs()
}

// OnStageStart records that a stage has begun executing.
func (t *Tracker) OnStageStart(signal StageTransition) {
	wf := t.wm.Get(signal.WorkflowID)
	if wf == nil {
		return
	}
	now := time.Now()
	wf.ActiveStages[signal.StageID] = &core.StageState{
		StageID:   signal.StageID,
		Backend:   signal.Backend,
		Model:     signal.Model,
		Tokens:    signal.ContextTokens,
		Status:    core.StageStatusActive,
		StartedAt: now,
	}
	wf.BackendUsage[signal.Backend] = &core.BackendCacheEntry{
		Backend:    signal.Backend,
		Model:      signal.Model,
		Tokens:     signal.ContextTokens,
		Preserved:  true,
		LastUpdate: now,
	}
	wf.LastActivityAt = now
}

// OnStageComplete records that a stage has finished executing.
func (t *Tracker) OnStageComplete(signal StageTransition) {
	wf := t.wm.Get(signal.WorkflowID)
	if wf == nil {
		return
	}
	now := time.Now()
	stage, exists := wf.ActiveStages[signal.StageID]
	if exists {
		stage.Status = core.StageStatusCompleted
		stage.Tokens = signal.ContextTokens
		wf.CompletedStages[signal.StageID] = stage
		delete(wf.ActiveStages, signal.StageID)
	}
	if entry, ok := wf.BackendUsage[signal.Backend]; ok {
		entry.Tokens = signal.ContextTokens
		entry.LastUpdate = now
	}
	wf.LastActivityAt = now
}

// MarkBackendFlushed marks a workflow's cache on a backend as no longer preserved.
func (t *Tracker) MarkBackendFlushed(workflowID, backend string) {
	wf := t.wm.Get(workflowID)
	if wf == nil {
		return
	}
	if entry, ok := wf.BackendUsage[backend]; ok {
		entry.Preserved = false
	}
}

// LastStageOnBackend returns the most recently completed stage for a workflow
// on a given backend, or nil if none.
func (t *Tracker) LastStageOnBackend(workflowID, backend string) *core.StageState {
	wf := t.wm.Get(workflowID)
	if wf == nil {
		return nil
	}
	var latest *core.StageState
	for _, s := range wf.CompletedStages {
		if s.Backend == backend {
			if latest == nil || s.StartedAt.After(latest.StartedAt) {
				latest = s
			}
		}
	}
	return latest
}

// RecordInflight marks a request as in-flight on a backend.
func (t *Tracker) RecordInflight(req core.InflightRequest) {
	t.mu.Lock()
	defer t.mu.Unlock()
	byBackend, ok := t.inflight[req.Backend]
	if !ok {
		byBackend = make(map[string]*core.InflightRequest)
		t.inflight[req.Backend] = byBackend
	}
	byBackend[req.RequestID] = &req
}

// ClearInflight removes an in-flight request.
func (t *Tracker) ClearInflight(backend, requestID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if byBackend, ok := t.inflight[backend]; ok {
		delete(byBackend, requestID)
	}
}

// InflightOnBackend returns the number of in-flight requests on a backend.
func (t *Tracker) InflightOnBackend(backend string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.inflight[backend])
}

// InflightWorkflowsOnBackend returns the set of workflow IDs with in-flight
// requests on a given backend.
func (t *Tracker) InflightWorkflowsOnBackend(backend string) map[string]struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	wfIDs := make(map[string]struct{})
	for _, req := range t.inflight[backend] {
		wfIDs[req.WorkflowID] = struct{}{}
	}
	return wfIDs
}

// WorkflowsWithPreservedCacheOnBackend returns workflow IDs that have preserved
// cache entries on the given backend, excluding any currently in-flight.
func (t *Tracker) WorkflowsWithPreservedCacheOnBackend(backend string) []string {
	t.mu.Lock()
	inflightWFs := make(map[string]struct{})
	for _, req := range t.inflight[backend] {
		inflightWFs[req.WorkflowID] = struct{}{}
	}
	t.mu.Unlock()

	var ids []string
	for _, wfID := range t.wm.ActiveWorkflowIDs() {
		if _, busy := inflightWFs[wfID]; busy {
			continue
		}
		wf := t.wm.Get(wfID)
		if wf == nil {
			continue
		}
		if entry, ok := wf.BackendUsage[backend]; ok && entry.Preserved {
			ids = append(ids, wfID)
		}
	}
	return ids
}
