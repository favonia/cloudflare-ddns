package pp

import (
	"fmt"
	"strings"
)

// ordinalSuffix returns the English ordinal suffix ("st", "nd", "rd", or "th").
func ordinalSuffix(n int) string {
	switch n % 100 {
	case 11, 12, 13:
		return "th"
	default:
		switch n % 10 {
		case 1:
			return "st"
		case 2:
			return "nd"
		case 3:
			return "rd"
		default:
			return "th"
		}
	}
}

// Ordinal returns the ordinal string for a positive integer (1st, 2nd, 3rd, etc.).
func Ordinal(n int) string { return fmt.Sprintf("%d%s", n, ordinalSuffix(n)) }

// Join joins words with commas.
func Join(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

// EnglishJoinOrEmptyLabel joins words using English punctuation and returns
// emptyLabel when items is empty. It uses the Oxford comma.
func EnglishJoinOrEmptyLabel(items []string, emptyLabel string) string {
	switch l := len(items); l {
	case 0:
		return emptyLabel
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

// EnglishJoinMapOrEmptyLabel applies f to each item and then joins the results
// with English punctuation, returning emptyLabel when items is empty.
func EnglishJoinMapOrEmptyLabel[T any](f func(t T) string, items []T, emptyLabel string) string {
	ss := make([]string, 0, len(items))
	for _, item := range items {
		ss = append(ss, f(item))
	}

	return EnglishJoinOrEmptyLabel(ss, emptyLabel)
}
