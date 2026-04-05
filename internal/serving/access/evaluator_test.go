package access

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluator_NoPolicies_AllowByDefault(t *testing.T) {
	e := NewEvaluator(NewStore())
	d := e.CheckAccess(map[string]string{"tenant": "anyone"}, ResourceTypeBackend, "gpt4o")
	assert.True(t, d.Allowed)
}

func TestEvaluator_DenyOverridesAllow(t *testing.T) {
	s := NewStore()
	require.NoError(t, s.Add(&Policy{
		Name: "allow-all", Subjects: []string{"tenant:*"}, Resources: []string{"backend:*"}, Action: ActionAllow,
	}))
	require.NoError(t, s.Add(&Policy{
		Name: "deny-gpt4o", Subjects: []string{"tenant:interns"}, Resources: []string{"backend:gpt4o"}, Action: ActionDeny,
	}))
	e := NewEvaluator(s)

	// Interns denied to gpt4o even though allow-all matches.
	d := e.CheckAccess(map[string]string{"tenant": "interns"}, ResourceTypeBackend, "gpt4o")
	assert.False(t, d.Allowed)
	assert.Contains(t, d.Reason, "deny-gpt4o")

	// Interns allowed to other backends.
	d = e.CheckAccess(map[string]string{"tenant": "interns"}, ResourceTypeBackend, "llama")
	assert.True(t, d.Allowed)

	// Research team allowed to gpt4o.
	d = e.CheckAccess(map[string]string{"tenant": "research"}, ResourceTypeBackend, "gpt4o")
	assert.True(t, d.Allowed)
}

func TestEvaluator_GlobPatterns(t *testing.T) {
	s := NewStore()
	require.NoError(t, s.Add(&Policy{
		Name: "deny-external", Subjects: []string{"workflow:prod-*"}, Resources: []string{"backend:ext-*"}, Action: ActionDeny,
	}))
	e := NewEvaluator(s)

	d := e.CheckAccess(map[string]string{"workflow": "prod-pipeline"}, ResourceTypeBackend, "ext-openai")
	assert.False(t, d.Allowed)

	d = e.CheckAccess(map[string]string{"workflow": "prod-pipeline"}, ResourceTypeBackend, "local-llama")
	assert.True(t, d.Allowed)

	d = e.CheckAccess(map[string]string{"workflow": "dev-test"}, ResourceTypeBackend, "ext-openai")
	assert.True(t, d.Allowed)
}

func TestEvaluator_ToolDeny(t *testing.T) {
	s := NewStore()
	require.NoError(t, s.Add(&Policy{
		Name: "deny-shell", Subjects: []string{"tenant:untrusted"}, Resources: []string{"tool:shell_*"}, Action: ActionDeny,
	}))
	e := NewEvaluator(s)

	tags := map[string]string{"tenant": "untrusted"}

	d := e.CheckAccess(tags, ResourceTypeTool, "search")
	assert.True(t, d.Allowed)

	d = e.CheckAccess(tags, ResourceTypeTool, "shell_exec")
	assert.False(t, d.Allowed)
	assert.Contains(t, d.Reason, "deny-shell")
}

func TestEvaluator_DataLabels(t *testing.T) {
	s := NewStore()
	require.NoError(t, s.Add(&Policy{
		Name: "pii-no-external", Subjects: []string{"backend:ext-*"}, Resources: []string{"data:pii"}, Action: ActionDeny,
	}))
	e := NewEvaluator(s)

	// External backend denied PII data.
	d := e.CheckAccess(map[string]string{"backend": "ext-openai"}, ResourceTypeData, "pii")
	assert.False(t, d.Allowed)

	// Internal backend allowed PII data.
	d = e.CheckAccess(map[string]string{"backend": "local-llama"}, ResourceTypeData, "pii")
	assert.True(t, d.Allowed)
}

func TestEvaluator_NoTagsAllowed(t *testing.T) {
	s := NewStore()
	require.NoError(t, s.Add(&Policy{
		Name: "deny-all", Subjects: []string{"tenant:*"}, Resources: []string{"backend:*"}, Action: ActionDeny,
	}))
	e := NewEvaluator(s)

	// Empty tags don't match "tenant:*", so access is allowed (open by default).
	d := e.CheckAccess(map[string]string{}, ResourceTypeBackend, "anything")
	assert.True(t, d.Allowed)
}
