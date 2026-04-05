package core

import "time"

// StageStatus tracks the lifecycle of a single stage.
type StageStatus string

const (
	StageStatusActive    StageStatus = "active"
	StageStatusCompleted StageStatus = "completed"
)

// StageState records per-stage metadata needed for cache and access control decisions.
type StageState struct {
	StageID   string
	Backend   string
	Model     string
	Tokens    int
	Status    StageStatus
	StartedAt time.Time
}

// BackendCacheEntry tracks a workflow's cache footprint on a specific backend.
type BackendCacheEntry struct {
	Backend    string
	Model      string
	Tokens     int
	Preserved  bool
	LastUpdate time.Time
}

// InflightRequest describes a request currently being processed by a backend worker.
type InflightRequest struct {
	RequestID  string
	WorkflowID string
	StageID    string
	Backend    string
	Streaming  bool
	StartedAt  time.Time
}

// WorkflowState is the shared representation of an active workflow, used by
// both the memory manager to make cache decisions and by access control to make
// data label propagation decisions.
type WorkflowState struct {
	ID              string
	ActiveStages    map[string]*StageState        // stageID -> state
	CompletedStages map[string]*StageState        // stageID -> state
	BackendUsage    map[string]*BackendCacheEntry // backend name -> cache entry
	StartedAt       time.Time
	LastActivityAt  time.Time

	// Edges maps a stage ID to its downstream stage IDs (the workflow DAG).
	Edges map[string][]string
	// DataLabels tracks accumulated data labels per stage (explicit + inherited).
	DataLabels map[string]map[string]struct{}
	// Declassifies records which labels each stage strips from propagation.
	// A stage that declassifies a label still inherits it (policies apply to
	// its own execution), but the label is not propagated to downstream stages.
	Declassifies map[string]map[string]struct{}
}

// NewWorkflowState creates a WorkflowState with all maps initialized.
func NewWorkflowState(id string) *WorkflowState {
	now := time.Now()
	return &WorkflowState{
		ID:              id,
		ActiveStages:    make(map[string]*StageState),
		CompletedStages: make(map[string]*StageState),
		BackendUsage:    make(map[string]*BackendCacheEntry),
		StartedAt:       now,
		LastActivityAt:  now,
		Edges:           make(map[string][]string),
		DataLabels:      make(map[string]map[string]struct{}),
		Declassifies:    make(map[string]map[string]struct{}),
	}
}

// EffectiveLabels records the given request labels on the stage, propagates
// them to all transitive downstream stages via the DAG edges, and returns
// the full accumulated label set for the stage including both explicit and inherited labels.
//
// The caller must hold any necessary locks, as this method is not thread-safe.
func (ws *WorkflowState) EffectiveLabels(stageID string, requestLabels []string) []string {
	// Record explicit labels and propagate downstream.
	if len(requestLabels) > 0 {
		set := ws.DataLabels[stageID]
		if set == nil {
			set = make(map[string]struct{})
			ws.DataLabels[stageID] = set
		}
		for _, l := range requestLabels {
			set[l] = struct{}{}
		}
		ws.propagate(stageID, set)
	}

	// Return accumulated labels for this stage.
	set := ws.DataLabels[stageID]
	if len(set) == 0 {
		return requestLabels
	}
	out := make([]string, 0, len(set))
	for l := range set {
		out = append(out, l)
	}
	return out
}

// propagate pushes labels to all transitive descendants in the DAG.
// When labels pass through a stage that declassifies them, those labels
// are removed from further propagation.
func (ws *WorkflowState) propagate(stageID string, labels map[string]struct{}) {
	// Filter out labels that this stage declassifies before propagating.
	outgoing := labels
	if declass := ws.Declassifies[stageID]; len(declass) > 0 {
		outgoing = make(map[string]struct{})
		for l := range labels {
			if _, stripped := declass[l]; !stripped {
				outgoing[l] = struct{}{}
			}
		}
		if len(outgoing) == 0 {
			return
		}
	}

	for _, child := range ws.Edges[stageID] {
		childSet := ws.DataLabels[child]
		if childSet == nil {
			childSet = make(map[string]struct{})
			ws.DataLabels[child] = childSet
		}
		added := false
		for l := range outgoing {
			if _, exists := childSet[l]; !exists {
				childSet[l] = struct{}{}
				added = true
			}
		}
		if added {
			ws.propagate(child, outgoing)
		}
	}
}
