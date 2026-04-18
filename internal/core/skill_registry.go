package core

import (
	"fmt"
	"sync"
)

// SkillRegistry stores skill manifests and provides thread-safe CRUD operations.
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]*SkillManifest
}

// NewSkillRegistry creates an empty skill registry.
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{skills: make(map[string]*SkillManifest)}
}

// Register stores a skill manifest. Returns an error if the manifest is invalid.
// Replaces any existing manifest with the same name.
func (r *SkillRegistry) Register(m *SkillManifest) error {
	if m.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if len(m.RequiresBackends) == 0 {
		return fmt.Errorf("skill %q: at least one required backend is needed", m.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[m.Name] = m
	return nil
}

// Get returns a skill manifest by name, or nil if not found.
func (r *SkillRegistry) Get(name string) *SkillManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.skills[name]
}

// List returns all registered skill manifests (snapshot).
func (r *SkillRegistry) List() []*SkillManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*SkillManifest, 0, len(r.skills))
	for _, m := range r.skills {
		out = append(out, m)
	}
	return out
}

// Remove deletes a skill manifest by name. Returns an error if not found.
func (r *SkillRegistry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.skills[name]; !ok {
		return fmt.Errorf("skill %q not found", name)
	}
	delete(r.skills, name)
	return nil
}
