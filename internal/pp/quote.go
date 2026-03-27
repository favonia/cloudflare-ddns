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

// QuoteIfUnsafeInSentence keeps simple sentence-safe ASCII tokens raw and
// quotes the rest so they can be embedded safely in English prose.
//
// The heuristic is positional rather than flat: some punctuation is fine at the
// start, more is fine in the middle, and only a small subset is fine at the
// end. This keeps common URLs, paths, regexes, dotfiles, fragments, env vars,
// and handles readable without letting raw tokens blend into prose punctuation.
// Use this for known token-like formats where preserving the raw shape improves
// readability. For opaque, malformed, or otherwise unknown literal data, prefer
// unconditional quoting at the call site instead of relying on this heuristic.
func QuoteIfUnsafeInSentence(s string) string {
	if isSafeInSentence(s) {
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

func isSafeInSentence(s string) bool {
	if s == "" {
		return false
	}
	runes := []rune(s)
	for i, r := range runes {
		if isASCIILetterOrDigit(r) {
			continue
		}
		switch {
		case i == 0 && i == len(runes)-1:
			return false
		case i == 0 && isSafeSentenceStartPunctuation(r):
		case i == len(runes)-1 && isSafeSentenceEndPunctuation(r):
		case i > 0 && i < len(runes)-1 && isSafeSentenceMiddlePunctuation(r):
		default:
			return false
		}
	}
	return true
}

func isASCIILetterOrDigit(r rune) bool {
	switch {
	case 'a' <= r && r <= 'z':
		return true
	case 'A' <= r && r <= 'Z':
		return true
	case '0' <= r && r <= '9':
		return true
	default:
		return false
	}
}

func isSafeSentenceStartPunctuation(r rune) bool {
	switch r {
	case '^', '/', '.', '#', '$', '@':
		return true
	default:
		return false
	}
}

func isSafeSentenceMiddlePunctuation(r rune) bool {
	switch r {
	case '.', '/', '\\', '#', '$', '%', '^', '&', '*', ':', '_', '~', '?', '=', '+', '@', '-':
		return true
	default:
		return false
	}
}

func isSafeSentenceEndPunctuation(r rune) bool {
	switch r {
	case '/', '\\', '$':
		return true
	default:
		return false
	}
}
