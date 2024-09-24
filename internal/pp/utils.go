package pp

import (
	"fmt"
	"strings"
)

// Join joins words with commas.
func Join(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

// EnglishJoin joins words as in English:
// - (none)
// - A
// - A and B
// - A, B, and C
// Note that we use Oxford commas.
func EnglishJoin(items []string) string {
	switch l := len(items); l {
	case 0:
		return "(none)"
	case 1:
		return items[0]
	case 2:
		return fmt.Sprintf("%s and %s", items[0], items[1])
	default:
		return fmt.Sprintf("%s, and %s", strings.Join(items[:l-1], ", "), items[l-1])
	}
}

// JoinMap applies a function to each element in the slice and then call Join.
func JoinMap[T any](f func(t T) string, items []T) string {
	ss := make([]string, 0, len(items))
	for _, item := range items {
		ss = append(ss, f(item))
	}

	return Join(ss)
}

// EnglishJoinMap applies a function to each element in the slice and then call EnglishJoin.
func EnglishJoinMap[T any](f func(t T) string, items []T) string {
	ss := make([]string, 0, len(items))
	for _, item := range items {
		ss = append(ss, f(item))
	}

	return EnglishJoin(ss)
}
