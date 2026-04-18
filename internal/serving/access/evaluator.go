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
//
// Evaluation follows four rules in order:
//  1. If any matching policy denies access, the request is denied, i.e., deny overrides allow.
//  2. If any matching policy allows access, the request is allowed.
//  3. If the subject is managed for this resource type (at least one policy targets
//     this subject with a resource of the same type, e.g., backend:*), but none
//     matched the specific resource, the request is denied.
//  4. If no policies target this subject for this resource type, the request is allowed.
//
// "Managed" is scoped per resource type: having backend policies does not affect
// tool access. Subjects without any policies are unaffected by access control.
// Once you install a policy for a subject and resource type, that combination
// needs explicit allows.
func (e *Evaluator) CheckAccess(tags map[string]string, resourceType ResourceType, resourceName string) Decision {
	resource := string(resourceType) + ":" + resourceName
	tagStrings := tagsToStrings(tags)

	var hasAllow, hasDeny bool
	var denyPolicy string
	subjectManagedForType := false
	typePrefix := string(resourceType) + ":"

	for _, p := range e.store.List() {
		if !MatchesAny(tagStrings, p.Subjects) {
			continue
		}
		// Check whether this policy targets the same resource type.
		// A subject is only "managed" for a given type if at least one
		// policy for that subject references that type (e.g., backend:*, tool:shell_*).
		if policyTargetsType(p, typePrefix) {
			subjectManagedForType = true
		}

		if !MatchesAny([]string{resource}, p.Resources) {
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

	// 1. Deny overrides allow.
	if hasDeny {
		return Decision{Allowed: false, Reason: "denied by policy " + denyPolicy}
	}

	// 2. Explicit allow.
	if hasAllow {
		return Decision{Allowed: true}
	}

	// 3. Subject is managed for this resource type but no policy matched.
	if subjectManagedForType {
		return Decision{Allowed: false, Reason: "no allow policy for this resource"}
	}

	// 4. Subject has no policies for this resource type.
	return Decision{Allowed: true}
}

// policyTargetsType returns true if any resource pattern in the policy starts
// with the given type prefix (e.g., "backend:", "tool:", "data:").
func policyTargetsType(p *Policy, typePrefix string) bool {
	for _, r := range p.Resources {
		if len(r) >= len(typePrefix) && r[:len(typePrefix)] == typePrefix {
			return true
		}
	}
	return false
}

// tagsToStrings converts a tag map to "key:value" strings for matching.
func tagsToStrings(tags map[string]string) []string {
	out := make([]string, 0, len(tags))
	for k, v := range tags {
		out = append(out, k+":"+v)
	}
	return out
}

// MatchesAny returns true if any value matches any pattern (glob).
func MatchesAny(values []string, patterns []string) bool {
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
