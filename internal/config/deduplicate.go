package config

import "slices"

// sortAndCompact sorts and deduplicates a list using built-in equality.
func sortAndCompact[T comparable](list []T, compare func(T, T) int) []T {
	slices.SortFunc(list, compare)
	return slices.Compact(list)
}
