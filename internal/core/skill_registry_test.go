package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillRegistry_RegisterAndList(t *testing.T) {
	r := NewSkillRegistry()
	require.NoError(t, r.Register(&SkillManifest{
		Name:             "summarize",
		RequiresBackends: []string{"cheap"},
	}))
	assert.Len(t, r.List(), 1)
	assert.Equal(t, "summarize", r.List()[0].Name)
}

func TestSkillRegistry_RegisterReplacesExisting(t *testing.T) {
	r := NewSkillRegistry()
	require.NoError(t, r.Register(&SkillManifest{Name: "s1", RequiresBackends: []string{"a"}}))
	require.NoError(t, r.Register(&SkillManifest{Name: "s1", RequiresBackends: []string{"b"}}))
	assert.Len(t, r.List(), 1)
	assert.Equal(t, []string{"b"}, r.Get("s1").RequiresBackends)
}

func TestSkillRegistry_Get(t *testing.T) {
	r := NewSkillRegistry()
	require.NoError(t, r.Register(&SkillManifest{Name: "s1", RequiresBackends: []string{"a"}}))
	assert.NotNil(t, r.Get("s1"))
	assert.Nil(t, r.Get("nonexistent"))
}

func TestSkillRegistry_Remove(t *testing.T) {
	r := NewSkillRegistry()
	require.NoError(t, r.Register(&SkillManifest{Name: "s1", RequiresBackends: []string{"a"}}))
	require.NoError(t, r.Remove("s1"))
	assert.Len(t, r.List(), 0)
}

func TestSkillRegistry_RemoveNotFound(t *testing.T) {
	r := NewSkillRegistry()
	assert.Error(t, r.Remove("nonexistent"))
}

func TestSkillRegistry_ValidationErrors(t *testing.T) {
	r := NewSkillRegistry()
	assert.Error(t, r.Register(&SkillManifest{Name: "", RequiresBackends: []string{"a"}}), "empty name")
	assert.Error(t, r.Register(&SkillManifest{Name: "s1", RequiresBackends: nil}), "no backends")
}
