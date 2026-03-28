package serving

import (
	"testing"

	"github.com/harvard-cns/orla/internal/core"
	"github.com/harvard-cns/orla/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectBackendByAccuracy_PicksCheapest(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	mgr.AddLLMBackend("expensive", &core.LLMBackend{
		Endpoint: "http://a",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.9),
		CostModel: &core.CostModel{
			InputCostPerMToken:  3.0,
			OutputCostPerMToken: 15.0,
		},
	}, "openai:big")
	mgr.AddLLMBackend("cheap", &core.LLMBackend{
		Endpoint: "http://b",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.7),
		CostModel: &core.CostModel{
			InputCostPerMToken:  0.25,
			OutputCostPerMToken: 1.25,
		},
	}, "openai:small")

	name, err := mgr.SelectBackendByAccuracy(0.5, model.AccuracyPolicyStrict, "")
	require.NoError(t, err)
	assert.Equal(t, "cheap", name)
}

func TestSelectBackendByAccuracy_FiltersLowQuality(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	mgr.AddLLMBackend("weak", &core.LLMBackend{
		Endpoint: "http://a",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.3),
		CostModel: &core.CostModel{
			InputCostPerMToken:  0.1,
			OutputCostPerMToken: 0.5,
		},
	}, "openai:tiny")
	mgr.AddLLMBackend("strong", &core.LLMBackend{
		Endpoint: "http://b",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.9),
		CostModel: &core.CostModel{
			InputCostPerMToken:  5.0,
			OutputCostPerMToken: 20.0,
		},
	}, "openai:big")

	name, err := mgr.SelectBackendByAccuracy(0.8, model.AccuracyPolicyStrict, "")
	require.NoError(t, err)
	assert.Equal(t, "strong", name)
}

func TestSelectBackendByAccuracy_NoneQualify_Strict(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	mgr.AddLLMBackend("low", &core.LLMBackend{
		Endpoint: "http://a",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.3),
		CostModel: &core.CostModel{
			InputCostPerMToken:  0.1,
			OutputCostPerMToken: 0.5,
		},
	}, "openai:tiny")

	_, err := mgr.SelectBackendByAccuracy(0.9, model.AccuracyPolicyStrict, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no backend with quality >= 0.9")
}

func TestSelectBackendByAccuracy_NoneQualify_PreferFallback(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	mgr.AddLLMBackend("low-cheap", &core.LLMBackend{
		Endpoint: "http://a",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.3),
		CostModel: &core.CostModel{
			InputCostPerMToken:  0.1,
			OutputCostPerMToken: 0.5,
		},
	}, "openai:tiny")
	mgr.AddLLMBackend("low-expensive", &core.LLMBackend{
		Endpoint: "http://b",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.2),
		CostModel: &core.CostModel{
			InputCostPerMToken:  5.0,
			OutputCostPerMToken: 20.0,
		},
	}, "openai:big")

	name, err := mgr.SelectBackendByAccuracy(0.9, model.AccuracyPolicyPrefer, "default-be")
	require.NoError(t, err)
	assert.Equal(t, "low-cheap", name, "prefer policy should fall back to cheapest costed backend")
}

func TestSelectBackendByAccuracy_PreferNoCostModel_ReturnsDefault(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	mgr.AddLLMBackend("no-cost", &core.LLMBackend{
		Endpoint: "http://a",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.9),
	}, "openai:no-cost")

	selected, err := mgr.SelectBackendByAccuracy(0.5, model.AccuracyPolicyPrefer, "my-default")
	require.NoError(t, err, "prefer should not error when no cost models exist")
	assert.Equal(t, "my-default", selected, "should return the caller's default backend")
}

func TestSelectBackendByAccuracy_SkipsNoCostModel(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	mgr.AddLLMBackend("no-cost", &core.LLMBackend{
		Endpoint: "http://a",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.9),
	}, "openai:no-cost")
	mgr.AddLLMBackend("with-cost", &core.LLMBackend{
		Endpoint: "http://b",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.7),
		CostModel: &core.CostModel{
			InputCostPerMToken:  1.0,
			OutputCostPerMToken: 5.0,
		},
	}, "openai:costed")

	name, err := mgr.SelectBackendByAccuracy(0.5, model.AccuracyPolicyStrict, "")
	require.NoError(t, err)
	assert.Equal(t, "with-cost", name)
}

func TestSelectBackendByAccuracy_EmptyBackends(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	_, err := mgr.SelectBackendByAccuracy(0.5, model.AccuracyPolicyStrict, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no backend")
}

func TestSelectBackendByAccuracy_ExactQualityBoundary(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	mgr.AddLLMBackend("exact", &core.LLMBackend{
		Endpoint: "http://a",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.7),
		CostModel: &core.CostModel{
			InputCostPerMToken:  1.0,
			OutputCostPerMToken: 5.0,
		},
	}, "openai:exact")

	name, err := mgr.SelectBackendByAccuracy(0.7, model.AccuracyPolicyStrict, "")
	require.NoError(t, err)
	assert.Equal(t, "exact", name)
}

func TestSelectBackendByAccuracy_DeterministicTieBreak(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	cm := &core.CostModel{InputCostPerMToken: 1.0, OutputCostPerMToken: 5.0}
	mgr.AddLLMBackend("bravo", &core.LLMBackend{
		Endpoint: "http://a", Type: core.LLMInferenceAPITypeOpenAI,
		Quality: core.Ptr(0.8), CostModel: cm,
	}, "openai:b")
	mgr.AddLLMBackend("alpha", &core.LLMBackend{
		Endpoint: "http://b", Type: core.LLMInferenceAPITypeOpenAI,
		Quality: core.Ptr(0.8), CostModel: cm,
	}, "openai:a")

	name, err := mgr.SelectBackendByAccuracy(0.5, model.AccuracyPolicyStrict, "")
	require.NoError(t, err)
	assert.Equal(t, "alpha", name, "tie should be broken by backend name (alphabetical)")
}

func TestSelectBackendByAccuracy_QualityNilExcluded(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	mgr.AddLLMBackend("unscored", &core.LLMBackend{
		Endpoint: "http://a",
		Type:     core.LLMInferenceAPITypeOpenAI,
		CostModel: &core.CostModel{
			InputCostPerMToken:  0.1,
			OutputCostPerMToken: 0.5,
		},
	}, "openai:unscored")

	_, err := mgr.SelectBackendByAccuracy(0, model.AccuracyPolicyStrict, "")
	require.Error(t, err, "quality=nil backends should be excluded as unscored")
}

func TestSelectBackendByAccuracy_QualityZeroIsValid(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	mgr.AddLLMBackend("zero-quality", &core.LLMBackend{
		Endpoint: "http://a",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.0),
		CostModel: &core.CostModel{
			InputCostPerMToken:  0.1,
			OutputCostPerMToken: 0.5,
		},
	}, "openai:zero")

	name, err := mgr.SelectBackendByAccuracy(0, model.AccuracyPolicyStrict, "")
	require.NoError(t, err, "quality=0.0 is a valid score and should participate in routing")
	assert.Equal(t, "zero-quality", name)
}

func TestSelectBackendByAccuracy_DefaultPolicyIsPrefer(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	mgr.AddLLMBackend("low", &core.LLMBackend{
		Endpoint: "http://a",
		Type:     core.LLMInferenceAPITypeOpenAI,
		Quality:  core.Ptr(0.3),
		CostModel: &core.CostModel{
			InputCostPerMToken:  0.1,
			OutputCostPerMToken: 0.5,
		},
	}, "openai:tiny")

	// Empty policy should behave like "prefer" — fallback to cheapest.
	name, err := mgr.SelectBackendByAccuracy(0.9, "", "default-be")
	require.NoError(t, err)
	assert.Equal(t, "low", name)
}

func TestGetCostModel(t *testing.T) {
	mgr := NewLLMBackendManager(nil)
	cm := &core.CostModel{InputCostPerMToken: 1.0, OutputCostPerMToken: 5.0}
	mgr.AddLLMBackend("b", &core.LLMBackend{
		Endpoint:  "http://a",
		Type:      core.LLMInferenceAPITypeOpenAI,
		CostModel: cm,
	}, "openai:m")

	got := mgr.GetCostModel("b")
	require.NotNil(t, got)
	assert.Equal(t, 1.0, got.InputCostPerMToken)
	assert.Equal(t, 5.0, got.OutputCostPerMToken)

	assert.Nil(t, mgr.GetCostModel("nonexistent"))
}
