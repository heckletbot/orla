// Package serving implements a minimal programmatic serving layer.
package serving

import (
	"context"
	"fmt"
	"maps"

	"github.com/harvard-cns/orla/internal/core"
	"github.com/harvard-cns/orla/internal/model"
	"github.com/harvard-cns/orla/internal/serving/access"
	"github.com/harvard-cns/orla/internal/serving/memory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// AgenticLayer is the serving layer that manages LLM backends and executes inference.
type AgenticLayer struct {
	llmBackendManager *LLMBackendManager
	MemoryManager     *memory.DefaultManager
	WorkflowManager   *core.WorkflowManager
	SkillRegistry     *core.SkillRegistry
	PolicyStore       *access.Store
	policyEvaluator   *access.Evaluator
}

// NewAgenticLayer creates a new serving layer.
func NewAgenticLayer() *AgenticLayer {
	wm := core.NewWorkflowManager()
	mm := memory.NewDefaultManager(memory.DefaultManagerConfig{}, wm)
	ps := access.NewStore()
	sr := core.NewSkillRegistry()
	return &AgenticLayer{
		llmBackendManager: NewLLMBackendManager(mm),
		MemoryManager:     mm,
		WorkflowManager:   wm,
		SkillRegistry:     sr,
		PolicyStore:       ps,
		policyEvaluator:   access.NewEvaluator(ps),
	}
}

// AddLLMBackend registers an LLM backend by name.
func (l *AgenticLayer) AddLLMBackend(name string, backend *core.LLMBackend, modelID string) {
	l.llmBackendManager.AddLLMBackend(name, backend, modelID)
}

// GetModelProvider returns the model provider for a named LLM backend.
func (l *AgenticLayer) GetModelProvider(ctx context.Context, backendName string) (model.Provider, error) {
	return l.llmBackendManager.GetModelProvider(ctx, backendName)
}

// Execute runs a single non-streaming inference call against the named LLM backend.
// For streaming, use ExecuteStream instead. opts.Stream must be false.
func (l *AgenticLayer) Execute(ctx context.Context, serverName, stageName string, messages []model.Message, tools []*mcp.Tool, opts model.InferenceOptions, chatOpts ...ChatOptions) (*model.Response, error) {
	if opts.Stream {
		return nil, fmt.Errorf("Execute does not support streaming, use ExecuteStream instead")
	}

	response, _, err := l.llmBackendManager.ScheduleChat(ctx, serverName, stageName, messages, tools, opts, chatOpts...)
	if err != nil {
		return nil, fmt.Errorf("inference failed on server '%s': %w", serverName, err)
	}
	zap.L().Debug("Executed inference",
		zap.String("server", serverName),
		zap.Int("response_length", len(response.Content)))
	return response, nil
}

// ExecuteStream runs inference with streaming. It returns the response (filled as the stream
// is consumed), a channel of stream events, and an error. The caller must consume the channel
// until closed; the response content, tool_calls, and metrics are populated by the provider's
// goroutine as the stream completes. opts.Stream must be true.
func (l *AgenticLayer) ExecuteStream(ctx context.Context, serverName, stageName string, messages []model.Message, tools []*mcp.Tool, opts model.InferenceOptions, chatOpts ...ChatOptions) (*model.Response, <-chan model.StreamEvent, error) {
	if !opts.Stream {
		return nil, nil, fmt.Errorf("ExecuteStream requires opts.Stream to be true")
	}

	response, ch, err := l.llmBackendManager.ScheduleChat(ctx, serverName, stageName, messages, tools, opts, chatOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("inference failed on server '%s': %w", serverName, err)
	}
	return response, ch, nil
}

// GetLLMBackendHealth returns the health status of a named LLM backend.
func (l *AgenticLayer) GetLLMBackendHealth(ctx context.Context, serverName string) (HealthStatus, error) {
	return l.llmBackendManager.GetHealthStatus(ctx, serverName)
}

// ListLLMBackends returns all registered LLM backend names.
func (l *AgenticLayer) ListLLMBackends() []string {
	return l.llmBackendManager.ListLLMBackends()
}

// SelectBackendByAccuracy picks the cheapest backend whose Quality >= accuracy.
// Policy controls fallback behavior (see AccuracyPolicyPrefer, AccuracyPolicyStrict).
func (l *AgenticLayer) SelectBackendByAccuracy(accuracy float64, policy string, defaultBackend string) (string, error) {
	return l.llmBackendManager.SelectBackendByAccuracy(accuracy, policy, defaultBackend)
}

// UpdateBackend applies a partial update to a registered backend.
func (l *AgenticLayer) UpdateBackend(name string, update BackendUpdate) error {
	return l.llmBackendManager.UpdateBackend(name, update)
}

// GetCostModel returns the CostModel for a registered backend, or nil if not found or unset.
func (l *AgenticLayer) GetCostModel(backendName string) *core.CostModel {
	return l.llmBackendManager.GetCostModel(backendName)
}

// NotifyWorkflowComplete emits TransitionWorkflowComplete signals for each
// backend the workflow used, then deregisters the workflow from the tracker.
func (l *AgenticLayer) NotifyWorkflowComplete(ctx context.Context, workflowID string, backends []string) {
	if l.MemoryManager == nil {
		return
	}
	for _, backend := range backends {
		l.MemoryManager.OnTransition(ctx, memory.StageTransition{
			TransitionType: memory.TransitionWorkflowComplete,
			WorkflowID:     workflowID,
			Backend:        backend,
		})
	}
	l.WorkflowManager.Deregister(workflowID)
}

// CheckBackendAccess checks whether the given tags permit access to the named backend.
func (l *AgenticLayer) CheckBackendAccess(tags map[string]string, backendName string) access.Decision {
	return l.policyEvaluator.CheckAccess(tags, access.ResourceTypeBackend, backendName)
}

// CheckToolAccess checks whether the given tags permit all requested tools.
// Returns the first denial encountered, or an allowed decision if all tools pass.
func (l *AgenticLayer) CheckToolAccess(tags map[string]string, tools []*mcp.Tool) access.Decision {
	for _, t := range tools {
		if d := l.policyEvaluator.CheckAccess(tags, access.ResourceTypeTool, t.Name); !d.Allowed {
			return d
		}
	}
	return access.Decision{Allowed: true}
}

// CheckDataAccess checks whether data with the given labels may be sent to the named backend.
func (l *AgenticLayer) CheckDataAccess(tags map[string]string, backendName string, dataLabels []string) access.Decision {
	for _, label := range dataLabels {
		// For data policies, the subject is the backend receiving the data,
		// and the resource is the data label.
		backendTags := map[string]string{"backend": backendName}
		// Merge request tags so policies can match on either.
		maps.Copy(backendTags, tags)

		if d := l.policyEvaluator.CheckAccess(backendTags, access.ResourceTypeData, label); !d.Allowed {
			return d
		}
	}
	return access.Decision{Allowed: true}
}

// CheckToolAccessByName checks whether the given tags permit a single tool by name.
func (l *AgenticLayer) CheckToolAccessByName(tags map[string]string, toolName string) access.Decision {
	return l.policyEvaluator.CheckAccess(tags, access.ResourceTypeTool, toolName)
}

// CheckSkillAccess checks whether the given tags permit invocation of the named skill.
func (l *AgenticLayer) CheckSkillAccess(tags map[string]string, skillName string) access.Decision {
	return l.policyEvaluator.CheckAccess(tags, access.ResourceTypeSkill, skillName)
}

// CheckSkillEnvelope verifies that the skill's manifest is within the intersection
// of three bounds: the manifest itself, skill-scoped policies, and the invoker's policies.
func (l *AgenticLayer) CheckSkillEnvelope(tags map[string]string, skillID string, manifest *core.SkillManifest) access.Decision {
	// Build tags augmented with the skill identifier for skill-scoped policy matching.
	skillTags := maps.Clone(tags)
	skillTags["skill"] = skillID

	// Check each required backend against invoker policies and skill-scoped policies.
	for _, backend := range manifest.RequiresBackends {
		if d := l.policyEvaluator.CheckAccess(tags, access.ResourceTypeBackend, backend); !d.Allowed {
			return access.Decision{Allowed: false, Reason: fmt.Sprintf("invoker cannot access backend %q: %s", backend, d.Reason)}
		}
		if d := l.policyEvaluator.CheckAccess(skillTags, access.ResourceTypeBackend, backend); !d.Allowed {
			return access.Decision{Allowed: false, Reason: fmt.Sprintf("skill-scoped policy denies backend %q: %s", backend, d.Reason)}
		}
	}

	// Check each required tool.
	for _, tool := range manifest.RequiresTools {
		if d := l.policyEvaluator.CheckAccess(tags, access.ResourceTypeTool, tool); !d.Allowed {
			return access.Decision{Allowed: false, Reason: fmt.Sprintf("invoker cannot access tool %q: %s", tool, d.Reason)}
		}
		if d := l.policyEvaluator.CheckAccess(skillTags, access.ResourceTypeTool, tool); !d.Allowed {
			return access.Decision{Allowed: false, Reason: fmt.Sprintf("skill-scoped policy denies tool %q: %s", tool, d.Reason)}
		}
	}

	// Check each required label against each required backend.
	for _, label := range manifest.RequiresLabels {
		for _, backend := range manifest.RequiresBackends {
			backendTags := map[string]string{"backend": backend}
			maps.Copy(backendTags, tags)
			if d := l.policyEvaluator.CheckAccess(backendTags, access.ResourceTypeData, label); !d.Allowed {
				return access.Decision{Allowed: false, Reason: fmt.Sprintf("data label %q denied for backend %q: %s", label, backend, d.Reason)}
			}
		}
	}

	return access.Decision{Allowed: true}
}

// CheckManifestBounds verifies that the actual request resources are within the skill's declared manifest.
func (l *AgenticLayer) CheckManifestBounds(manifest *core.SkillManifest, backendName string, toolNames []string, dataLabels []string) access.Decision {
	// Backend must be in the manifest.
	if !access.MatchesAny([]string{backendName}, manifest.RequiresBackends) {
		return access.Decision{Allowed: false, Reason: fmt.Sprintf("backend %q not in skill manifest", backendName)}
	}

	// Each tool must be in the manifest. Empty manifest means no tools allowed.
	for _, tool := range toolNames {
		if !access.MatchesAny([]string{tool}, manifest.RequiresTools) {
			return access.Decision{Allowed: false, Reason: fmt.Sprintf("tool %q not in skill manifest", tool)}
		}
	}

	// Each data label must be in the manifest. Empty manifest means no labeled data allowed.
	for _, label := range dataLabels {
		if !access.MatchesAny([]string{label}, manifest.RequiresLabels) {
			return access.Decision{Allowed: false, Reason: fmt.Sprintf("data label %q not in skill manifest", label)}
		}
	}

	return access.Decision{Allowed: true}
}

// ValidateAccess runs all access control checks for a request and returns the
// first denial, or an allowed decision if everything passes. Both handleExecute
// and handleAccessCheck call this method.
//
// If skillID is non-empty, the skill must be registered. The method performs
// skill visibility, envelope, and manifest bounds checks before the standard
// backend/tool/data checks. On success with a skill, the returned tags map
// includes the injected "skill" tag for downstream policy matching.
func (l *AgenticLayer) ValidateAccess(
	tags map[string]string,
	backend string,
	toolNames []string,
	dataLabels []string,
	skillID string,
) (access.Decision, map[string]string) {
	// Skill checks.
	if skillID != "" {
		manifest := l.SkillRegistry.Get(skillID)
		if manifest == nil {
			return access.Decision{Allowed: false, Reason: fmt.Sprintf("skill %q not registered", skillID)}, tags
		}
		if len(tags) > 0 {
			if d := l.CheckSkillAccess(tags, skillID); !d.Allowed {
				return access.Decision{Allowed: false, Reason: fmt.Sprintf("access denied to skill %q: %s", skillID, d.Reason)}, tags
			}
			if d := l.CheckSkillEnvelope(tags, skillID, manifest); !d.Allowed {
				return access.Decision{Allowed: false, Reason: fmt.Sprintf("skill %q envelope check failed: %s", skillID, d.Reason)}, tags
			}
		}
		if d := l.CheckManifestBounds(manifest, backend, toolNames, dataLabels); !d.Allowed {
			return access.Decision{Allowed: false, Reason: fmt.Sprintf("skill %q manifest violation: %s", skillID, d.Reason)}, tags
		}
		// Inject skill tag for downstream policy matching.
		if tags != nil {
			tags = maps.Clone(tags)
			tags["skill"] = skillID
		}
	}

	// Backend check.
	if backend != "" {
		if d := l.CheckBackendAccess(tags, backend); !d.Allowed {
			return access.Decision{Allowed: false, Reason: fmt.Sprintf("access denied to backend %q: %s", backend, d.Reason)}, tags
		}
	}

	// Tool checks.
	for _, tool := range toolNames {
		if d := l.CheckToolAccessByName(tags, tool); !d.Allowed {
			return access.Decision{Allowed: false, Reason: fmt.Sprintf("tool access denied: %s", d.Reason)}, tags
		}
	}

	// Data label checks.
	if backend != "" && len(dataLabels) > 0 {
		if d := l.CheckDataAccess(tags, backend, dataLabels); !d.Allowed {
			return access.Decision{Allowed: false, Reason: fmt.Sprintf("data access denied for backend %q: %s", backend, d.Reason)}, tags
		}
	}

	return access.Decision{Allowed: true}, tags
}

// StartPressureMonitor launches the background memory pressure polling loop.
// It dynamically queries the current set of backends on each tick and stops
// when ctx is cancelled.
func (l *AgenticLayer) StartPressureMonitor(ctx context.Context) {
	go l.MemoryManager.StartPressureMonitor(ctx, l.llmBackendManager.ListLLMBackends, 0)
}
