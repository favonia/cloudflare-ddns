// Package sliceutil contains common helpers for slice operations.
package sliceutil

import "slices"

// SortAndCompact sorts and deduplicates a list using built-in equality.
func SortAndCompact[T comparable](list []T, compare func(T, T) int) []T {
	slices.SortFunc(list, compare)
	return slices.Compact(list)
}
