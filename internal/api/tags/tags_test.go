package tags_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	apitags "github.com/favonia/cloudflare-ddns/internal/api/tags"
)

func TestResolveDuplicateKeepsLexicographicallySmallest(t *testing.T) {
	t.Parallel()

	resolved := apitags.Resolve([][]string{{"name:value", "Name:value", "x:two"}})

	require.Equal(t, []string{"Name:value", "x:two"}, resolved.Inherited)
	require.Nil(t, resolved.Dropped)
	require.True(t, resolved.HasDuplicateCanonical)
	require.False(t, resolved.HasAmbiguousCanonical)
}

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("empty-outdated-returns-empty", func(t *testing.T) {
		t.Parallel()
		resolved := apitags.Resolve(nil)
		require.Nil(t, resolved.Inherited)
		require.Nil(t, resolved.Dropped)
		require.False(t, resolved.HasAmbiguousCanonical)
		require.False(t, resolved.HasDuplicateCanonical)
	})

	t.Run("returns-unanimous-tags-only", func(t *testing.T) {
		t.Parallel()
		outdated := [][]string{
			{"x:new", "y:drop"},
			{"X:new", "z:drop"},
			{"x:new"},
		}
		resolved := apitags.Resolve(outdated)
		require.Equal(t, []string{"X:new"}, resolved.Inherited)
		require.Equal(t, []string{"y:drop", "z:drop"}, resolved.Dropped)
		require.True(t, resolved.HasAmbiguousCanonical)
		require.False(t, resolved.HasDuplicateCanonical)
	})

	t.Run("name-case-insensitive-value-case-sensitive", func(t *testing.T) {
		t.Parallel()
		outdated := [][]string{
			{"name:Value", "hi:Sigma"},
			{"NAME:Value", "HI:sigma"},
		}

		resolved := apitags.Resolve(outdated)
		require.Equal(t, []string{"NAME:Value"}, resolved.Inherited)
		require.Equal(t, []string{"hi:Sigma", "HI:sigma"}, resolved.Dropped)
	})

	t.Run("returns-empty-when-no-canonical-tag-is-unanimous", func(t *testing.T) {
		t.Parallel()
		outdated := [][]string{
			{"env:prod"},
			{"team:alpha"},
		}
		resolved := apitags.Resolve(outdated)
		require.Nil(t, resolved.Inherited)
		require.Equal(t, []string{"env:prod", "team:alpha"}, resolved.Dropped)
		require.True(t, resolved.HasAmbiguousCanonical)
	})
}

func TestResolveOrderInvariant(t *testing.T) {
	t.Parallel()

	outdated := [][]string{
		{"A:1", "x:one"},
		{"A:1", "x:one"},
		{"a:1", "x:one"},
	}
	original := apitags.Resolve(outdated)

	permutedOutdated := [][]string{
		{"x:one", "a:1"},
		{"x:one", "A:1"},
		{"x:one", "A:1"},
	}
	permuted := apitags.Resolve(permutedOutdated)
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
	require.True(t, apitags.Equal(
		[]string{"FeatureFlag", "NAME:value"},
		[]string{"featureflag", "name:value"},
	))
}

func TestResolveTracksDroppedAndDuplicateCanonicals(t *testing.T) {
	t.Parallel()

	resolved := apitags.Resolve([][]string{
		{"NAME:one", "name:one", "x:two"},
		{"name:one", "X:two"},
	})

	require.Equal(t, []string{"NAME:one", "X:two"}, resolved.Inherited)
	require.Nil(t, resolved.Dropped)
	require.False(t, resolved.HasAmbiguousCanonical)
	require.True(t, resolved.HasDuplicateCanonical)
}

func TestResolveAmbiguousCanonical(t *testing.T) {
	t.Parallel()

	t.Run("detects-mismatched-canonical-key-sets", func(t *testing.T) {
		t.Parallel()
		resolved := apitags.Resolve([][]string{
			{"env:prod", "team:alpha"},
			{"env:prod"},
		})
		require.Equal(t, []string{"env:prod"}, resolved.Inherited)
		require.Equal(t, []string{"team:alpha"}, resolved.Dropped)
		require.True(t, resolved.HasAmbiguousCanonical)
	})

	t.Run("ignores-representation-only-differences", func(t *testing.T) {
		t.Parallel()
		resolved := apitags.Resolve([][]string{
			{"TEAM:alpha", "Env:prod"},
			{"team:alpha", "env:prod"},
		})
		require.Equal(t, []string{"Env:prod", "TEAM:alpha"}, resolved.Inherited)
		require.False(t, resolved.HasAmbiguousCanonical)
		require.False(t, resolved.HasDuplicateCanonical)
	})
}

func TestResolveOrderInvariantAcrossTagSets(t *testing.T) {
	t.Parallel()

	tagSets := [][]string{
		{"TEAM:alpha", "env:prod"},
		{"team:alpha", "env:prod"},
	}
	original := apitags.Resolve(tagSets)

	permutedTagSets := slices.Clone(tagSets)
	slices.Reverse(permutedTagSets)
	permuted := apitags.Resolve(permutedTagSets)

	require.Equal(t, original, permuted)
}

func TestResolveUndocumentedTags(t *testing.T) {
	t.Parallel()

	resolved := apitags.Resolve([][]string{
		{"FeatureFlag", "NAME:value"},
		{"featureflag", "name:value"},
	})

	require.Equal(t, []string{"FeatureFlag", "NAME:value"}, resolved.Inherited)
	require.Nil(t, resolved.Dropped)
	require.False(t, resolved.HasAmbiguousCanonical)
	require.False(t, resolved.HasDuplicateCanonical)
}

func TestUndocumented(t *testing.T) {
	t.Parallel()

	require.Nil(t, apitags.Undocumented(nil))
	require.Nil(t, apitags.Undocumented([]string{"env:prod", "team:", "Name:Value"}))
	require.Equal(t,
		[]string{"featureflag", ":prod", ""},
		apitags.Undocumented([]string{"env:prod", "featureflag", ":prod", "", "team:"}),
	)
}
