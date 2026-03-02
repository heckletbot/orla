package orla

import (
	"context"
	"fmt"
)

// AgentStage holds backend, inference options, output format, and tools for a phase.
// The agent uses the current stage when building requests.
type AgentStage struct {
	Name string
	// LLMBackend is the backend used for inference.
	LLMBackend *LLMBackend
	// MaxTokens is optional; nil means backend default.
	MaxTokens *int
	// Temperature is optional; nil means backend default.
	Temperature *float64
	// TopP is optional; nil means backend default.
	TopP *float64
	// ResponseFormat requests structured output (JSON Schema). Nil means no structured output.
	ResponseFormat *StructuredOutputRequest
	// ChatTemplateKwargs are extra kwargs passed to the chat template renderer (e.g. SGLang/vLLM).
	ChatTemplateKwargs map[string]any
	// SchedulingPolicy configures server-side backend queue scheduling for requests in this stage.
	SchedulingPolicy string
	// SchedulingHints are optional policy hints for this stage.
	SchedulingHints *SchedulingHints
	// Tools are the tools available in this stage (e.g. different stages can expose different tool sets).
	Tools map[string]*Tool
}

// NewAgentStage returns a stage with the given backend; other options are nil (backend defaults), Tools is empty.
func NewAgentStage(name string, backend *LLMBackend) *AgentStage {
	return &AgentStage{Name: name, LLMBackend: backend, Tools: make(map[string]*Tool)}
}

// SetMaxTokens sets the maximum tokens for execute calls (nil means backend default).
func (s *AgentStage) SetMaxTokens(n int) { s.MaxTokens = &n }

// SetTemperature sets the sampling temperature for execute calls (nil means backend default).
func (s *AgentStage) SetTemperature(f float64) { s.Temperature = &f }

// SetTopP sets the nucleus sampling top_p for execute calls (nil means backend default).
func (s *AgentStage) SetTopP(f float64) { s.TopP = &f }

// SetResponseFormat sets the structured output (JSON Schema) for execute calls. Use nil to disable.
func (s *AgentStage) SetResponseFormat(r *StructuredOutputRequest) { s.ResponseFormat = r }

// SetChatTemplateKwargs sets extra kwargs for the chat template renderer
func (s *AgentStage) SetChatTemplateKwargs(kwargs map[string]any) { s.ChatTemplateKwargs = kwargs }

// SetSchedulingPolicy configures server-side scheduling policy for this stage's requests.
func (s *AgentStage) SetSchedulingPolicy(policy string) { s.SchedulingPolicy = policy }

// SetSchedulingHints sets optional server scheduling hints for this stage.
func (s *AgentStage) SetSchedulingHints(hints *SchedulingHints) { s.SchedulingHints = hints }

// AddTool adds a tool to this stage. Returns an error if t is nil.
func (s *AgentStage) AddTool(t *Tool) error {
	if t == nil {
		return fmt.Errorf("tool cannot be nil")
	}
	s.Tools[t.Name] = t
	return nil
}

// StageMapper maps a prompt to an execution stage.
type StageMapper interface {
	MapStage(ctx context.Context, prompt string) (*AgentStage, error)
}

// OneBitStageMapper is a stage mapper that uses a one bit predictor and a prompt to do stage mapping.
type OneBitStageMapper struct {
	OneBitPredictor *OneBitPredictor
	StageOne        *AgentStage
	StageTwo        *AgentStage
	Prompt          string
}

// NewOneBitStageMapper returns a new one bit stage mapper.
func NewOneBitStageMapper(client *OrlaClient, backend *LLMBackend, stageOne *AgentStage, stageTwo *AgentStage) *OneBitStageMapper {
	return &OneBitStageMapper{OneBitPredictor: NewOneBitPredictor(client, backend), StageOne: stageOne, StageTwo: stageTwo}
}

// MapStage maps the stage based on the prompt.
func (m *OneBitStageMapper) MapStage(ctx context.Context, prompt string) (*AgentStage, error) {
	prediction, err := m.OneBitPredictor.Predict(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to predict stage: %w", err)
	}

	if prediction {
		return m.StageOne, nil
	}

	return m.StageTwo, nil
}

// PromptScorer computes a routing score for a prompt.
type PromptScorer func(prompt string) float64

// ThresholdStageMapper routes prompts to one of two stages by comparing score to a threshold.
type ThresholdStageMapper struct {
	Threshold float64
	LowStage  *AgentStage
	HighStage *AgentStage
	ScoreFn   PromptScorer
}

// NewThresholdStageMapper creates a stage mapper that routes by score threshold.
func NewThresholdStageMapper(threshold float64, lowStage, highStage *AgentStage, scoreFn PromptScorer) *ThresholdStageMapper {
	return &ThresholdStageMapper{
		Threshold: threshold,
		LowStage:  lowStage,
		HighStage: highStage,
		ScoreFn:   scoreFn,
	}
}

// MapStage maps prompt to stage based on score threshold.
func (m *ThresholdStageMapper) MapStage(_ context.Context, prompt string) (*AgentStage, error) {
	scoreFn := m.ScoreFn
	if scoreFn == nil {
		scoreFn = func(p string) float64 { return float64(len(p)) }
	}
	if scoreFn(prompt) >= m.Threshold {
		return m.HighStage, nil
	}
	return m.LowStage, nil
}
