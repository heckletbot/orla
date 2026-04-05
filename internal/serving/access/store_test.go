package access

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_AddAndList(t *testing.T) {
	s := NewStore()
	p := &Policy{Name: "p1", Subjects: []string{"tenant:a"}, Resources: []string{"backend:x"}, Action: ActionDeny}
	require.NoError(t, s.Add(p))
	assert.Len(t, s.List(), 1)
	assert.Equal(t, "p1", s.List()[0].Name)
}

func TestStore_AddReplacesExisting(t *testing.T) {
	s := NewStore()
	p1 := &Policy{Name: "p1", Subjects: []string{"tenant:a"}, Resources: []string{"backend:x"}, Action: ActionDeny}
	p2 := &Policy{Name: "p1", Subjects: []string{"tenant:b"}, Resources: []string{"backend:y"}, Action: ActionAllow}
	require.NoError(t, s.Add(p1))
	require.NoError(t, s.Add(p2))
	assert.Len(t, s.List(), 1)
	assert.Equal(t, []string{"tenant:b"}, s.Get("p1").Subjects)
}

func TestStore_Remove(t *testing.T) {
	s := NewStore()
	require.NoError(t, s.Add(&Policy{Name: "p1", Subjects: []string{"t:a"}, Resources: []string{"backend:x"}, Action: ActionDeny}))
	require.NoError(t, s.Remove("p1"))
	assert.Len(t, s.List(), 0)
}

func TestStore_RemoveNotFound(t *testing.T) {
	s := NewStore()
	assert.Error(t, s.Remove("nonexistent"))
}

func TestStore_ValidationErrors(t *testing.T) {
	s := NewStore()

	assert.Error(t, s.Add(&Policy{Name: "", Subjects: []string{"a"}, Resources: []string{"b"}, Action: ActionDeny}), "empty name")
	assert.Error(t, s.Add(&Policy{Name: "p", Subjects: nil, Resources: []string{"b"}, Action: ActionDeny}), "no subjects")
	assert.Error(t, s.Add(&Policy{Name: "p", Subjects: []string{"a"}, Resources: nil, Action: ActionDeny}), "no resources")
	assert.Error(t, s.Add(&Policy{Name: "p", Subjects: []string{"a"}, Resources: []string{"b"}, Action: "invalid"}), "bad action")
}
