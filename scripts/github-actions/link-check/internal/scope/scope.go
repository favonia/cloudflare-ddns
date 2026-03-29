// Package scope selects tracked repository files for individual link-check
// passes.
package scope

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// SourceFilesConfig configures which tracked repository files belong to one
// check's input scope.
type SourceFilesConfig struct {
	// IncludeGlobs selects tracked repository files that this check should scan.
	IncludeGlobs []string
	// ExcludeGlobs removes tracked files from IncludeGlobs matches before
	// extraction begins.
	ExcludeGlobs []string
}

type compiledPatterns struct {
	regexps []*regexp.Regexp
}

func newCompiledPatterns(patterns []string) compiledPatterns {
	regexps := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		regexps = append(regexps, regexp.MustCompile(globToRegexp(pattern)))
	}
	return compiledPatterns{regexps: regexps}
}

func (patterns compiledPatterns) match(candidate string) bool {
	for _, compiled := range patterns.regexps {
		if compiled.MatchString(candidate) {
			return true
		}
	}
	return false
}

func globToRegexp(pattern string) string {
	var builder strings.Builder
	builder.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				builder.WriteString(".*")
				i++
				continue
			}
			builder.WriteString(`[^/]*`)
		case '?':
			builder.WriteString(`[^/]`)
		default:
			builder.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	builder.WriteString("$")
	return builder.String()
}

// TrackedFiles lists tracked repository paths under root using `git ls-files`.
func TrackedFiles(root string) ([]string, error) {
	command := exec.CommandContext(
		context.Background(), "git", "ls-files", "-z",
	)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("run git ls-files: %w", err)
	}
	rawPaths := bytes.Split(output, []byte{0})
	files := make([]string, 0, len(rawPaths))
	for _, rawPath := range rawPaths {
		if len(rawPath) == 0 {
			continue
		}
		files = append(files, filepath.ToSlash(string(rawPath)))
	}
	return files, nil
}

// FilterFiles applies include and exclude globs to tracked repository paths.
func FilterFiles(files, includePatterns, excludePatterns []string) []string {
	include := newCompiledPatterns(includePatterns)
	exclude := newCompiledPatterns(excludePatterns)
	selected := make([]string, 0, len(files))
	for _, file := range files {
		if len(includePatterns) > 0 && !include.match(file) {
			continue
		}
		if len(excludePatterns) > 0 && exclude.match(file) {
			continue
		}
		selected = append(selected, file)
	}
	return selected
}
