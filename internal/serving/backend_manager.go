package serving

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dorcha-inc/orla/internal/core"
	"github.com/dorcha-inc/orla/internal/model"
	"go.uber.org/zap"
)

type backendEntry struct {
	backend *core.LLMBackend
	modelID string
}

// LLMBackendManager manages a pool of LLM backend configurations and their providers
type LLMBackendManager struct {
	backends  map[string]*backendEntry
	providers map[string]model.Provider
	mu        sync.RWMutex
}

// NewLLMBackendManager creates a new LLM backend manager
func NewLLMBackendManager() *LLMBackendManager {
	return &LLMBackendManager{
		backends:  make(map[string]*backendEntry),
		providers: make(map[string]model.Provider),
	}
}

// AddLLMBackend registers an LLM backend by name
func (m *LLMBackendManager) AddLLMBackend(name string, backend *core.LLMBackend, modelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backends[name] = &backendEntry{backend: backend, modelID: modelID}
	delete(m.providers, name)
}

// GetModelProvider returns a cached provider for an LLM backend, creating it if necessary
func (m *LLMBackendManager) GetModelProvider(ctx context.Context, backendName string) (model.Provider, error) {
	m.mu.RLock()
	if provider, exists := m.providers[backendName]; exists {
		m.mu.RUnlock()
		return provider, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if provider, exists := m.providers[backendName]; exists {
		return provider, nil
	}

	entry, exists := m.backends[backendName]
	if !exists {
		return nil, fmt.Errorf("llm_backend '%s' not found", backendName)
	}

	provider, err := model.NewProviderFromBackend(entry.backend, entry.modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider for llm_backend '%s': %w", backendName, err)
	}

	m.providers[backendName] = provider

	zap.L().Debug("Created and cached a model provider for LLM backend",
		zap.String("backend_name", backendName),
		zap.String("model", entry.modelID))

	return provider, nil
}

// HealthStatus represents the health status of an LLM backend
type HealthStatus string

const (
	HealthStatusHealthy     HealthStatus = "healthy"
	HealthStatusDegraded    HealthStatus = "degraded"
	HealthStatusUnavailable HealthStatus = "unavailable"
)

const (
	healthCheckTimeout           = 5 * time.Second
	healthCheckDegradedThreshold = 2 * time.Second
)

// GetHealthStatus returns the health status of an LLM backend
func (m *LLMBackendManager) GetHealthStatus(ctx context.Context, backendName string) (HealthStatus, error) {
	m.mu.RLock()
	_, exists := m.backends[backendName]
	m.mu.RUnlock()
	if !exists {
		return HealthStatusUnavailable, fmt.Errorf("llm_backend '%s' not found", backendName)
	}

	provider, err := m.GetModelProvider(ctx, backendName)
	if err != nil {
		return HealthStatusUnavailable, fmt.Errorf("failed to get provider: %w", err)
	}

	healthCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	start := time.Now()
	err = provider.EnsureReady(healthCtx)
	duration := time.Since(start)

	if healthCtx.Err() == context.DeadlineExceeded {
		return HealthStatusUnavailable, fmt.Errorf("health check timed out after %v", healthCheckTimeout)
	}

	if err != nil {
		return HealthStatusUnavailable, fmt.Errorf("health check failed: %w", err)
	}

	if duration > healthCheckDegradedThreshold {
		return HealthStatusDegraded, nil
	}

	return HealthStatusHealthy, nil
}

// ListLLMBackends returns a list of all LLM backend names
func (m *LLMBackendManager) ListLLMBackends() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backendNames := make([]string, 0, len(m.backends))
	for backendName := range m.backends {
		backendNames = append(backendNames, backendName)
	}
	return backendNames
}
