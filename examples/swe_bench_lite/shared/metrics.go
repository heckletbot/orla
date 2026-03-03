package shared

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// InstanceMetrics is the timing and token usage for one SWE-bench instance (single-shot).
type InstanceMetrics struct {
	InstanceID          string `json:"instance_id"`
	MappedStage         string `json:"mapped_stage,omitempty"`
	Priority            int    `json:"priority,omitempty"`
	QueuePosition       int    `json:"queue_position,omitempty"`
	PromptLength        int    `json:"prompt_length"`
	StartTime           int64  `json:"start_time_ms"`
	EndTime             int64  `json:"end_time_ms"`
	DurationMs          int64  `json:"duration_ms"`
	PromptTokens        int    `json:"prompt_tokens,omitempty"`
	CompletionTokens    int    `json:"completion_tokens,omitempty"`
	QueueWaitMs         int64  `json:"queue_wait_ms,omitempty"`
	SchedulerDecisionMs int64  `json:"scheduler_decision_ms,omitempty"`
	DispatchMs          int64  `json:"dispatch_ms,omitempty"`
	BackendLatencyMs    int64  `json:"backend_latency_ms,omitempty"`
	TTFTMs              int64  `json:"ttft_ms,omitempty"`
	TPOTMs              int64  `json:"tpot_ms,omitempty"`
}

// RunMetrics is the full run: end-to-end time, token usage, and per-instance metrics.
type RunMetrics struct {
	ExperimentName        string            `json:"experiment_name"`
	StartTime             int64             `json:"start_time_ms"`
	EndTime               int64             `json:"end_time_ms"`
	TotalDurationMs       int64             `json:"total_duration_ms"`
	Instances             []InstanceMetrics `json:"instances"`
	InstancesCount        int               `json:"instances_count"`
	TotalPromptTokens     int               `json:"total_prompt_tokens"`
	TotalCompletionTokens int               `json:"total_completion_tokens"`
}

// RunMetricsRecorder records timings for a run. Thread-safe for concurrent AddInstance calls.
type RunMetricsRecorder struct {
	ExperimentName string
	startTime      time.Time
	endTime        time.Time
	instances      []InstanceMetrics
	mu             sync.Mutex
}

// NewRunMetricsRecorder returns a new recorder.
func NewRunMetricsRecorder(experimentName string) *RunMetricsRecorder {
	return &RunMetricsRecorder{ExperimentName: experimentName}
}

// BeginRun records the run start time.
func (r *RunMetricsRecorder) BeginRun() {
	r.startTime = time.Now()
}

// EndRun records the run end time.
func (r *RunMetricsRecorder) EndRun() {
	r.endTime = time.Now()
	log.Printf("[metrics] run: duration=%dms", r.endTime.Sub(r.startTime).Milliseconds())
}

// AddInstance appends instance metrics. Thread-safe.
func (r *RunMetricsRecorder) AddInstance(inst InstanceMetrics) {
	r.mu.Lock()
	defer r.mu.Unlock()
	log.Printf("[metrics] instance %s: stage=%s, duration=%dms, prompt_tokens=%d, completion_tokens=%d, queue_wait=%dms, ttft=%dms",
		inst.InstanceID, inst.MappedStage, inst.DurationMs,
		inst.PromptTokens, inst.CompletionTokens,
		inst.QueueWaitMs, inst.TTFTMs)
	r.instances = append(r.instances, inst)
}

// Write writes the collected metrics to path. Uses MetricsPath if path is empty.
func (r *RunMetricsRecorder) Write(path string) error {
	if path == "" {
		path = os.Getenv("METRICS_PATH")
		if path == "" {
			path = MetricsPath
		}
	}
	endMs := r.endTime.UnixMilli()
	if r.endTime.IsZero() {
		endMs = time.Now().UnixMilli()
	}
	startMs := r.startTime.UnixMilli()

	var totalPrompt, totalCompletion int
	for _, inst := range r.instances {
		totalPrompt += inst.PromptTokens
		totalCompletion += inst.CompletionTokens
	}

	m := RunMetrics{
		ExperimentName:        r.ExperimentName,
		StartTime:             startMs,
		EndTime:               endMs,
		TotalDurationMs:       endMs - startMs,
		Instances:             r.instances,
		InstancesCount:        len(r.instances),
		TotalPromptTokens:     totalPrompt,
		TotalCompletionTokens: totalCompletion,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write metrics: %w", err)
	}
	return nil
}
