package external

import (
	"net/url"
	"strings"
)

type warningSuppressor func(probeResult) bool

var warningSuppressors = []warningSuppressor{
	suppressTrailingSlashRedirectWarning,
}

// shouldSuppressWarning applies policy hooks that intentionally hide specific
// low-value warning classes from operator output.
func shouldSuppressWarning(result probeResult) bool {
	for _, suppressor := range warningSuppressors {
		if suppressor(result) {
			return true
		}
	}
	return false
}

// suppressTrailingSlashRedirectWarning hides simple path-normalization
// redirects so external-link output stays focused on actionable degradation.
func suppressTrailingSlashRedirectWarning(result probeResult) bool {
	if len(result.Hops) != 2 {
		return false
	}
	first := result.Hops[0]
	second := result.Hops[1]
	if !redirectStatusCodes[first.Status] || second.Status == 0 {
		return false
	}

	from, err := url.Parse(first.URL)
	if err != nil {
		return false
	}
	to, err := url.Parse(second.URL)
	if err != nil {
		return false
	}
	if from.Scheme != to.Scheme || from.Host != to.Host {
		return false
	}
	if from.RawQuery != to.RawQuery || from.Fragment != to.Fragment {
		return false
	}
	if !pathsDifferOnlyByTrailingSlash(from.Path, to.Path) {
		return false
	}
	return true
}

// pathsDifferOnlyByTrailingSlash reports whether two URL paths differ only by a
// single trailing slash normalization.
func pathsDifferOnlyByTrailingSlash(fromPath, toPath string) bool {
	return toPath == fromPath+"/" || (strings.HasSuffix(fromPath, "/") && strings.TrimSuffix(fromPath, "/") == toPath)
}
