package access

import (
	"path/filepath"
)

// Decision is the result of policy evaluation.
type Decision struct {
	// Allowed is true when the request is permitted.
	Allowed bool
	// Reason describes why the request was denied. This is empty when
	// the request was allowed.
	Reason string
}

// Evaluator checks access control policies from a Store.
type Evaluator struct {
	store *Store
}

// NewEvaluator creates an evaluator backed by the given store.
func NewEvaluator(store *Store) *Evaluator {
	return &Evaluator{store: store}
}

// CheckAccess evaluates whether the given tags (subjects) may access a resource.
// It uses deny-overrides: if any matching policy denies access, the request is denied
// regardless of allow rules. If no policies match, access is allowed by default (open policy).
func (e *Evaluator) CheckAccess(tags map[string]string, resourceType ResourceType, resourceName string) Decision {
	resource := string(resourceType) + ":" + resourceName
	tagStrings := tagsToStrings(tags)

	var hasAllow, hasDeny bool
	var denyPolicy string

	for _, p := range e.store.List() {
		if !matchesAny(tagStrings, p.Subjects) {
			continue
		}
		if !matchesAny([]string{resource}, p.Resources) {
			continue
		}
		// Policy matches both subject and resource.
		switch p.Action {
		case ActionDeny:
			hasDeny = true
			denyPolicy = p.Name
		case ActionAllow:
			hasAllow = true
		}
	}

	// Deny overrides allow.
	if hasDeny {
		return Decision{Allowed: false, Reason: "denied by policy " + denyPolicy}
	}

	// If at least one allow matched, or no policies matched at all (open by default), permit.
	_ = hasAllow
	return Decision{Allowed: true}
}

// tagsToStrings converts a tag map to "key:value" strings for matching.
func tagsToStrings(tags map[string]string) []string {
	out := make([]string, 0, len(tags))
	for k, v := range tags {
		out = append(out, k+":"+v)
	}
	return out
}

// matchesAny returns true if any value matches any pattern (glob).
func matchesAny(values []string, patterns []string) bool {
	for _, v := range values {
		for _, p := range patterns {
			matched, err := filepath.Match(p, v)
			if err != nil {
				continue // invalid pattern, skip
			}
			if matched {
				return true
			}
		}
	}
	return false
}
