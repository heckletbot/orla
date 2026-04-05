package access

import (
	"fmt"
	"sync"
)

// Store is a thread-safe in-memory store for access control policies.
type Store struct {
	mu       sync.RWMutex
	policies map[string]*Policy // keyed by name
}

// NewStore creates an empty policy store.
func NewStore() *Store {
	return &Store{policies: make(map[string]*Policy)}
}

// Add inserts or replaces a policy. Returns an error if the policy is invalid.
func (s *Store) Add(p *Policy) error {
	if p.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	if len(p.Subjects) == 0 {
		return fmt.Errorf("policy %q: at least one subject is required", p.Name)
	}
	if len(p.Resources) == 0 {
		return fmt.Errorf("policy %q: at least one resource is required", p.Name)
	}
	if p.Action != ActionAllow && p.Action != ActionDeny {
		return fmt.Errorf("policy %q: action must be %q or %q; got %q", p.Name, ActionAllow, ActionDeny, p.Action)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[p.Name] = p
	return nil
}

// Remove deletes a policy by name. Returns an error if not found.
func (s *Store) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.policies[name]; !ok {
		return fmt.Errorf("policy %q not found", name)
	}
	delete(s.policies, name)
	return nil
}

// List returns all policies (snapshot).
func (s *Store) List() []*Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Policy, 0, len(s.policies))
	for _, p := range s.policies {
		out = append(out, p)
	}
	return out
}

// Get returns a single policy by name, or nil if not found.
func (s *Store) Get(name string) *Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.policies[name]
}
