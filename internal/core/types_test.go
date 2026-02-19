package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLLMInferenceAPIType_String(t *testing.T) {
	assert.Equal(t, "ollama", string(LLMInferenceAPITypeOllama))
	assert.Equal(t, "openai", string(LLMInferenceAPITypeOpenAI))
	assert.Equal(t, "sglang", string(LLMInferenceAPITypeSGLang))
}

func TestLLMBackend_Empty(t *testing.T) {
	backend := &LLMBackend{}
	assert.Empty(t, backend.Endpoint)
	assert.Empty(t, backend.Type)
	assert.Empty(t, backend.APIKeyEnvVar)
}

func TestLLMBackend_WithValues(t *testing.T) {
	backend := &LLMBackend{
		Endpoint:     "http://localhost:8080",
		Type:         LLMInferenceAPITypeOllama,
		APIKeyEnvVar: "API_KEY",
	}

	assert.Equal(t, "http://localhost:8080", backend.Endpoint)
	assert.Equal(t, LLMInferenceAPITypeOllama, backend.Type)
	assert.Equal(t, "API_KEY", backend.APIKeyEnvVar)
}
