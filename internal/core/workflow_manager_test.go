package core

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sortedStrings(s []string) []string {
	sort.Strings(s)
	return s
}

func TestWorkflowManager_RegisterAndGet(t *testing.T) {
	wm := NewWorkflowManager()
	wm.Register("wf-1")
	assert.NotNil(t, wm.Get("wf-1"))
	assert.Nil(t, wm.Get("wf-unknown"))
}

func TestWorkflowManager_Deregister(t *testing.T) {
	wm := NewWorkflowManager()
	wm.Register("wf-1")
	wm.Deregister("wf-1")
	assert.Nil(t, wm.Get("wf-1"))
}

func TestWorkflowManager_RegisterEdges(t *testing.T) {
	wm := NewWorkflowManager()
	wm.RegisterEdges("wf-1", [][2]string{{"a", "b"}, {"b", "c"}})

	ws := wm.Get("wf-1")
	assert.NotNil(t, ws)
	assert.Equal(t, []string{"b"}, ws.Edges["a"])
	assert.Equal(t, []string{"c"}, ws.Edges["b"])
}

func TestWorkflowManager_EffectiveLabels_UnregisteredWorkflow(t *testing.T) {
	wm := NewWorkflowManager()
	_, err := wm.EffectiveLabels("unknown", "a", []string{"pii"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestWorkflowManager_EffectiveLabels_PropagatesAlongEdges(t *testing.T) {
	// DAG: a → b → c
	wm := NewWorkflowManager()
	wm.RegisterEdges("wf-1", [][2]string{{"a", "b"}, {"b", "c"}})

	labels, err := wm.EffectiveLabels("wf-1", "a", []string{"pii"})
	require.NoError(t, err)
	assert.Equal(t, []string{"pii"}, labels)

	labels, err = wm.EffectiveLabels("wf-1", "b", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"pii"}, sortedStrings(labels))

	labels, err = wm.EffectiveLabels("wf-1", "c", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"pii"}, sortedStrings(labels))
}

func TestWorkflowManager_EffectiveLabels_NoSidewaysPropagation(t *testing.T) {
	// DAG: a → c, b → d (two independent paths)
	wm := NewWorkflowManager()
	wm.RegisterEdges("wf-1", [][2]string{{"a", "c"}, {"b", "d"}})

	_, err := wm.EffectiveLabels("wf-1", "a", []string{"pii"})
	require.NoError(t, err)

	labels, err := wm.EffectiveLabels("wf-1", "b", nil)
	require.NoError(t, err)
	assert.Empty(t, labels)

	labels, err = wm.EffectiveLabels("wf-1", "d", nil)
	require.NoError(t, err)
	assert.Empty(t, labels)

	labels, err = wm.EffectiveLabels("wf-1", "c", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"pii"}, sortedStrings(labels))
}

func TestWorkflowManager_EffectiveLabels_MultiParentMerge(t *testing.T) {
	// DAG: a → c, b → c
	wm := NewWorkflowManager()
	wm.RegisterEdges("wf-1", [][2]string{{"a", "c"}, {"b", "c"}})

	_, err := wm.EffectiveLabels("wf-1", "a", []string{"pii"})
	require.NoError(t, err)
	_, err = wm.EffectiveLabels("wf-1", "b", []string{"confidential"})
	require.NoError(t, err)

	labels, err := wm.EffectiveLabels("wf-1", "c", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"confidential", "pii"}, sortedStrings(labels))
}

func TestWorkflowManager_EffectiveLabels_ExplicitMergesWithInherited(t *testing.T) {
	// DAG: a → b
	wm := NewWorkflowManager()
	wm.RegisterEdges("wf-1", [][2]string{{"a", "b"}})

	_, err := wm.EffectiveLabels("wf-1", "a", []string{"pii"})
	require.NoError(t, err)

	labels, err := wm.EffectiveLabels("wf-1", "b", []string{"financial"})
	require.NoError(t, err)
	assert.Equal(t, []string{"financial", "pii"}, sortedStrings(labels))
}

func TestWorkflowManager_Declassification(t *testing.T) {
	// DAG: extract → sanitize → summarize
	// sanitize declassifies "pii", so summarize should NOT inherit it.
	wm := NewWorkflowManager()
	wm.RegisterEdges("wf-1", [][2]string{{"extract", "sanitize"}, {"sanitize", "summarize"}})
	wm.RegisterDeclassifications("wf-1", map[string][]string{
		"sanitize": {"pii"},
	})

	// extract runs with PII.
	_, err := wm.EffectiveLabels("wf-1", "extract", []string{"pii"})
	require.NoError(t, err)

	// sanitize still inherits PII (it needs to process the data).
	labels, err := wm.EffectiveLabels("wf-1", "sanitize", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"pii"}, sortedStrings(labels))

	// summarize does NOT inherit PII — sanitize stripped it.
	labels, err = wm.EffectiveLabels("wf-1", "summarize", nil)
	require.NoError(t, err)
	assert.Empty(t, labels)
}

func TestWorkflowManager_Declassification_PartialStrip(t *testing.T) {
	// DAG: a → b → c. b declassifies "pii" but not "confidential".
	wm := NewWorkflowManager()
	wm.RegisterEdges("wf-1", [][2]string{{"a", "b"}, {"b", "c"}})
	wm.RegisterDeclassifications("wf-1", map[string][]string{
		"b": {"pii"},
	})

	_, err := wm.EffectiveLabels("wf-1", "a", []string{"pii", "confidential"})
	require.NoError(t, err)

	// b inherits both.
	labels, err := wm.EffectiveLabels("wf-1", "b", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"confidential", "pii"}, sortedStrings(labels))

	// c inherits only confidential — pii was declassified by b.
	labels, err = wm.EffectiveLabels("wf-1", "c", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"confidential"}, sortedStrings(labels))
}
