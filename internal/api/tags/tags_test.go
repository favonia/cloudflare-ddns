package tags_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	apitags "github.com/favonia/cloudflare-ddns/internal/api/tags"
)

func TestSummarizeSetsDuplicateKeepsLexicographicallySmallest(t *testing.T) {
	t.Parallel()

	summary := apitags.SummarizeSets([][]string{{"name:value", "Name:value", "x:two"}})

	require.Equal(t, 1, summary.SetCount)
	require.True(t, summary.HasDuplicateCanonical)
	require.False(t, summary.HasAmbiguousCanonical)
	require.Equal(t, "Name:value", summary.Representative["name:value"])
	require.Equal(t, "x:two", summary.Representative["x:two"])
	require.Equal(t, 1, summary.Occurrence["name:value"])
	require.Equal(t, 1, summary.Occurrence["x:two"])
}

func TestCommonSubset(t *testing.T) {
	t.Parallel()

	t.Run("empty-stale-returns-empty", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, apitags.CommonSubset(nil))
	})

	t.Run("returns-unanimous-tags-only", func(t *testing.T) {
		t.Parallel()
		stale := [][]string{
			{"x:new", "y:drop"},
			{"X:new", "z:drop"},
			{"x:new"},
		}
		require.Equal(t, []string{"X:new"}, apitags.CommonSubset(stale))
	})

	t.Run("name-case-insensitive-value-case-sensitive", func(t *testing.T) {
		t.Parallel()
		stale := [][]string{
			{"name:Value", "hi:Sigma"},
			{"NAME:Value", "HI:sigma"},
		}

		resolved := apitags.CommonSubset(stale)
		require.Contains(t, resolved, "NAME:Value")
		require.NotContains(t, resolved, "hi:Sigma")
		require.NotContains(t, resolved, "HI:sigma")
	})
}

func TestCommonSubsetOrderInvariant(t *testing.T) {
	t.Parallel()

	stale := [][]string{
		{"A:1", "x:one"},
		{"A:1", "x:one"},
		{"a:1", "x:one"},
	}
	original := apitags.CommonSubset(stale)

	permutedStale := [][]string{
		{"x:one", "a:1"},
		{"x:one", "A:1"},
		{"x:one", "A:1"},
	}
	permuted := apitags.CommonSubset(permutedStale)
	require.Equal(t, original, permuted)
}

func TestEqual(t *testing.T) {
	t.Parallel()
	require.True(t, apitags.Equal(
		[]string{"NAME:value", "x:Two"},
		[]string{"name:value", "X:Two"},
	))
	require.True(t, apitags.Equal(
		[]string{"NAME:value", "name:value", "x:Two"},
		[]string{"name:value", "X:Two", "x:Two"},
	))
	require.False(t, apitags.Equal(
		[]string{"name:Value"},
		[]string{"name:value"},
	))
}

func TestSummarizeSets(t *testing.T) {
	t.Parallel()

	summary := apitags.SummarizeSets([][]string{
		{"NAME:one", "name:one", "x:two"},
		{"name:one", "X:two"},
	})

	require.Equal(t, 2, summary.SetCount)
	require.Equal(t, 2, summary.Occurrence["name:one"])
	require.Equal(t, 2, summary.Occurrence["x:two"])
	require.Equal(t, "NAME:one", summary.Representative["name:one"])
	require.Equal(t, "X:two", summary.Representative["x:two"])
	require.False(t, summary.HasAmbiguousCanonical)
	require.True(t, summary.HasDuplicateCanonical)
}

func TestSummarizeSetsAmbiguousCanonical(t *testing.T) {
	t.Parallel()

	t.Run("detects-mismatched-canonical-key-sets", func(t *testing.T) {
		t.Parallel()
		summary := apitags.SummarizeSets([][]string{
			{"env:prod", "team:alpha"},
			{"env:prod"},
		})
		require.True(t, summary.HasAmbiguousCanonical)
	})

	t.Run("ignores-representation-only-differences", func(t *testing.T) {
		t.Parallel()
		summary := apitags.SummarizeSets([][]string{
			{"TEAM:alpha", "Env:prod"},
			{"team:alpha", "env:prod"},
		})
		require.False(t, summary.HasAmbiguousCanonical)
	})
}

func TestSummarizeSetsOrderInvariant(t *testing.T) {
	t.Parallel()

	tagSets := [][]string{
		{"TEAM:alpha", "env:prod"},
		{"team:alpha", "env:prod"},
	}
	original := apitags.SummarizeSets(tagSets)

	permutedTagSets := slices.Clone(tagSets)
	slices.Reverse(permutedTagSets)
	permuted := apitags.SummarizeSets(permutedTagSets)

	require.Equal(t, original, permuted)
}
