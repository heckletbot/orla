package model

import (
	"fmt"
	"strings"

	"github.com/dorcha-inc/orla/internal/config"
	"github.com/dorcha-inc/orla/internal/core"
)

// ParseModelIdentifier parses a model identifier string (e.g., "ollama:llama3")
// and returns the provider name and model name
func ParseModelIdentifier(modelID string) (provider, modelName string, err error) {
	parts := strings.SplitN(modelID, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid model identifier format: expected 'provider:model-name', got '%s'", modelID)
	}
	return parts[0], parts[1], nil
}

// NewProvider creates a new model provider based on the configuration
func NewProvider(cfg *config.OrlaConfig) (Provider, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("model not configured")
	}

	return newProviderForModel(cfg.Model, cfg.LLMBackend, cfg)
}

// NewProviderFromBackend creates a new model provider from a backend and model identifier.
// This is the programmatic entry point used by the serving layer.
func NewProviderFromBackend(backend *core.LLMBackend, modelID string) (Provider, error) {
	if modelID == "" {
		return nil, fmt.Errorf("model identifier is required")
	}

	cfg := &config.OrlaConfig{
		LLMBackend: backend,
		Model:      modelID,
	}

	return newProviderForModel(modelID, backend, cfg)
}

func newProviderForModel(modelID string, backend *core.LLMBackend, cfg *config.OrlaConfig) (Provider, error) {
	providerName, modelName, err := ParseModelIdentifier(modelID)
	if err != nil {
		return nil, err
	}

	supportedProviders := map[core.LLMInferenceAPIType]struct{}{
		core.LLMInferenceAPITypeOllama: {},
		core.LLMInferenceAPITypeOpenAI: {},
		core.LLMInferenceAPITypeSGLang: {},
	}

	switch providerName {
	case string(core.LLMInferenceAPITypeOllama):
		return NewOllamaProvider(modelName, cfg)
	case string(core.LLMInferenceAPITypeSGLang):
		if backend == nil {
			cfg.LLMBackend = &core.LLMBackend{Type: core.LLMInferenceAPITypeOllama}
		} else if backend.Type == "" {
			cfg.LLMBackend = &core.LLMBackend{
				Type:     core.LLMInferenceAPITypeOllama,
				Endpoint: backend.Endpoint,
			}
		} else if backend.Type != core.LLMInferenceAPITypeOllama {
			return nil, fmt.Errorf("for an SGLang backend, the Inference API type must be %s, got %s", core.LLMInferenceAPITypeOllama, backend.Type)
		}
		return NewOllamaProvider(modelName, cfg)
	case string(core.LLMInferenceAPITypeOpenAI):
		return NewOpenAIProvider(modelName, cfg.LLMBackend)
	default:
		return nil, fmt.Errorf("unknown model provider: %s: supported providers are %s", providerName, core.JoinMapKeys(supportedProviders))
	}
}
