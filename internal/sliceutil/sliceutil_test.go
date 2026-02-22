package sliceutil

import (
	"cmp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortAndCompactInt(t *testing.T) {
	t.Parallel()

	got := SortAndCompact([]int{4, 3, 4, 2, 1, 3, 2}, cmp.Compare[int])
	require.Equal(t, []int{1, 2, 3, 4}, got)
}

func TestSortAndCompactStruct(t *testing.T) {
	t.Parallel()

	type item struct {
		Priority int
		Name     string
	}

	compareItem := func(a, b item) int {
		return cmp.Or(
			cmp.Compare(a.Priority, b.Priority),
			cmp.Compare(a.Name, b.Name),
		)
	}

	got := SortAndCompact([]item{
		{Priority: 2, Name: "beta"},
		{Priority: 1, Name: "alpha"},
		{Priority: 2, Name: "beta"},
		{Priority: 2, Name: "gamma"},
		{Priority: 1, Name: "alpha"},
	}, compareItem)

	require.Equal(t, []item{
		{Priority: 1, Name: "alpha"},
		{Priority: 2, Name: "beta"},
		{Priority: 2, Name: "gamma"},
	}, got)
}

func TestSortAndCompactNilAndEmpty(t *testing.T) {
	t.Parallel()

	var nilInput []int
	require.Nil(t, SortAndCompact(nilInput, cmp.Compare[int]))
	require.Empty(t, SortAndCompact([]int{}, cmp.Compare[int]))
}
