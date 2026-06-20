package domainentry

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

// These trees never escape the grammar in practice; the tests pin the
// defensive panics and rejects that guard the parser-to-builder contract.

// unknownOp is an operator node whose form ID matches no grammar rule, used to
// drive the unrecognized-form default arms.
func unknownOp() syntax.Op[formID] {
	return syntax.Op[formID]{ID: formID("?"), Tokens: nil, Args: nil}
}

func newBuildState() *buildState {
	return &buildState{entries: nil, diagnostics: nil, extraComma: false, missingComma: false}
}

func TestMustAtomPanicsOnNonAtom(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() { mustAtom(syntax.EmptyTree[formID]{}) })
}

func TestMustOpPanicsOnNonOp(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() { mustOp(syntax.EmptyTree[formID]{}) })
}

func TestBuildEntryPanicsOnUnexpectedTree(t *testing.T) {
	t.Parallel()

	state := newBuildState()
	require.Panics(t, func() { _, _ = state.buildEntry(syntax.EmptyTree[formID]{}) })
}

func TestBuildFieldsPanicsOnUnexpectedForm(t *testing.T) {
	t.Parallel()

	state := newBuildState()
	require.Panics(t, func() { _, _ = state.buildFields(unknownOp()) })
}

func TestBuildListPanicsOnUnexpectedForm(t *testing.T) {
	t.Parallel()

	state := newBuildState()
	require.Panics(t, func() { _ = state.buildList(unknownOp()) })
}

func TestBuildListPanicsOnUnexpectedTree(t *testing.T) {
	t.Parallel()

	state := newBuildState()
	require.Panics(t, func() { _ = state.buildList(nil) })
}

func TestBuildHostID6ValuesPanicsOnUnexpectedForm(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() { _, _ = buildHostID6Values(unknownOp()) })
}

func TestBuildHostID6ValuesPanicsOnUnexpectedTree(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() { _, _ = buildHostID6Values(syntax.EmptyTree[formID]{}) })
}

func TestValidateValueRejectsUnexpectedTree(t *testing.T) {
	t.Parallel()

	// An empty value is unreachable via Parse, so the defensive default routes
	// to invalidTree, which in turn panics while locating a non-existent token.
	require.Panics(t, func() { _ = validateValue(syntax.EmptyTree[formID]{}) })
}

func TestIsPlainListRejectsUnexpectedTree(t *testing.T) {
	t.Parallel()

	require.False(t, isPlainList(syntax.EmptyTree[formID]{}))
}
