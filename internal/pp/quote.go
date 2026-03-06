package pp

import (
	"strconv"
	"strings"
	"unicode"
)

// QuoteOrEmptyLabel quotes non-empty strings and keeps a caller-defined label
// for the empty string.
func QuoteOrEmptyLabel(s string, emptyLabel string) string {
	if s == "" {
		return emptyLabel
	}
	return strconv.Quote(s)
}

// QuoteIfNotHumanReadable keeps human-readable single-line strings raw and
// quotes the rest to keep escaping explicit.
func QuoteIfNotHumanReadable(s string) string {
	if isHumanReadableSingleLine(s) {
		return s
	}
	return strconv.Quote(s)
}

// QuotePreview quotes a preview string and truncates it (rune-safe) when it
// is longer than the given limit. Non-positive limits disable truncation.
func QuotePreview(s string, limit int) string {
	preview, _ := truncateForPreview(s, limit)
	return strconv.Quote(preview)
}

// QuotePreviewIfNotHumanReadable keeps short human-readable strings raw, but
// quotes non-human-readable previews or truncated previews.
func QuotePreviewIfNotHumanReadable(s string, limit int) string {
	preview, truncated := truncateForPreview(s, limit)
	if truncated {
		return strconv.Quote(preview)
	}
	return QuoteIfNotHumanReadable(preview)
}

func truncateForPreview(s string, limit int) (string, bool) {
	if limit <= 0 {
		return s, false
	}

	runes := []rune(s)
	if len(runes) <= limit {
		return s, false
	}

	return string(runes[:limit]) + "...", true
}

func isHumanReadableSingleLine(s string) bool {
	if strings.TrimSpace(s) != s {
		return false
	}
	for _, r := range s {
		if unicode.IsGraphic(r) || r == ' ' {
			continue
		}
		return false
	}
	return true
}
