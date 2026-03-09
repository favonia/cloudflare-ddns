package setter //nolint:testpackage // Tests intentionally access unexported reconciliation helpers.

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveScalarValue(t *testing.T) {
	t.Parallel()

	value, ambiguous := resolveScalarValue("default", nil)
	require.Equal(t, "default", value)
	require.False(t, ambiguous)

	value, ambiguous = resolveScalarValue("default", []string{"same", "same", "same"})
	require.Equal(t, "same", value)
	require.False(t, ambiguous)

	value, ambiguous = resolveScalarValue("default", []string{"a", "b"})
	require.Equal(t, "default", value)
	require.True(t, ambiguous)
}

func TestResolveScalarValueOrderInvariant(t *testing.T) {
	t.Parallel()

	configured := "configured"
	input := []string{"z", "a", "z", "z"}

	valueA, ambiguousA := resolveScalarValue(configured, input)
	slices.Reverse(input)
	valueB, ambiguousB := resolveScalarValue(configured, input)

	require.Equal(t, valueA, valueB)
	require.Equal(t, ambiguousA, ambiguousB)
}
