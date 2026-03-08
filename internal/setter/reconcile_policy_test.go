package setter

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

func TestCommonTags(t *testing.T) {
	t.Parallel()

	t.Run("empty-stale-returns-empty", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, commonTags(nil))
	})

	t.Run("returns-unanimous-tags-only", func(t *testing.T) {
		t.Parallel()
		stale := [][]string{
			{"x:new", "y:drop"},
			{"X:new", "z:drop"},
			{"x:new"},
		}
		require.Equal(t, []string{"X:new"}, commonTags(stale))
	})

	t.Run("name-case-insensitive-value-case-sensitive", func(t *testing.T) {
		t.Parallel()
		stale := [][]string{
			{"name:Value", "hi:Sigma"},
			{"NAME:Value", "HI:sigma"},
		}

		resolved := commonTags(stale)
		require.Contains(t, resolved, "NAME:Value")
		// value case differs; they are distinct tags and not unanimous additions.
		require.NotContains(t, resolved, "hi:Sigma")
		require.NotContains(t, resolved, "HI:sigma")
	})
}

func TestCommonTagsOrderInvariant(t *testing.T) {
	t.Parallel()

	stale := [][]string{
		{"A:1", "x:one"},
		{"A:1", "x:one"},
		{"a:1", "x:one"},
	}
	original := commonTags(stale)

	permutedStale := [][]string{
		{"x:one", "a:1"},
		{"x:one", "A:1"},
		{"x:one", "A:1"},
	}
	permuted := commonTags(permutedStale)
	require.Equal(t, original, permuted)
}

func TestSameTagsByPolicy(t *testing.T) {
	t.Parallel()
	require.True(t, sameTagsByPolicy(
		[]string{"NAME:value", "x:Two"},
		[]string{"name:value", "X:Two"},
	))
	require.True(t, sameTagsByPolicy(
		[]string{"NAME:value", "name:value", "x:Two"},
		[]string{"name:value", "X:Two", "x:Two"},
	))
	require.False(t, sameTagsByPolicy(
		[]string{"name:Value"},
		[]string{"name:value"},
	))
}

func TestSummarizeTagSets(t *testing.T) {
	t.Parallel()

	summary := summarizeTagSets([][]string{
		{"NAME:one", "name:one", "x:two"},
		{"name:one", "X:two"},
	})

	require.Equal(t, 2, summary.setCount)
	require.Equal(t, 2, summary.occurrence["name:one"]) // duplicate tags in one set count once.
	require.Equal(t, 2, summary.occurrence["x:two"])
	require.Equal(t, "NAME:one", summary.representative["name:one"])
	require.Equal(t, "X:two", summary.representative["x:two"])
	require.False(t, summary.hasAmbiguousCanonical)
	require.True(t, summary.hasDuplicateCanonical)
}

func TestSummarizeTagSetsAmbiguousCanonical(t *testing.T) {
	t.Parallel()

	t.Run("detects-mismatched-canonical-key-sets", func(t *testing.T) {
		t.Parallel()
		summary := summarizeTagSets([][]string{
			{"env:prod", "team:alpha"},
			{"env:prod"},
		})
		require.True(t, summary.hasAmbiguousCanonical)
	})

	t.Run("ignores-representation-only-differences", func(t *testing.T) {
		t.Parallel()
		summary := summarizeTagSets([][]string{
			{"TEAM:alpha", "Env:prod"},
			{"team:alpha", "env:prod"},
		})
		require.False(t, summary.hasAmbiguousCanonical)
	})
}
