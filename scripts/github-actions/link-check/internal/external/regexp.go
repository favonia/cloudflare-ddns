package external

import "regexp"

// compileRegexps compiles full regular expressions used for extracted-target
// filtering and warning policy.
func compileRegexps(patterns []string) []*regexp.Regexp {
	regexps := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		regexps = append(regexps, regexp.MustCompile(pattern))
	}
	return regexps
}

// anyRegexpMatch reports whether any compiled expression matches candidate.
func anyRegexpMatch(regexps []*regexp.Regexp, candidate string) bool {
	for _, compiled := range regexps {
		if compiled.MatchString(candidate) {
			return true
		}
	}
	return false
}
