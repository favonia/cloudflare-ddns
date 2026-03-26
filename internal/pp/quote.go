package pp

import (
	"strconv"
	"strings"
	"unicode"
)

// AdvisoryPreviewLimit is the centralized advisory preview limit.
// Keep advisory value previews short in warning logs while preserving
// full-fidelity values for mismatch diagnostics.
const AdvisoryPreviewLimit = 48

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

// QuotePreviewOrEmptyLabel quotes a non-empty preview string and truncates it
// in a rune-safe way when it exceeds the given limit. A non-positive limit
// disables truncation. For an empty string, it returns the caller-defined
// label unchanged.
func QuotePreviewOrEmptyLabel(s string, limit int, emptyLabel string) string {
	if s == "" {
		return emptyLabel
	}
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
