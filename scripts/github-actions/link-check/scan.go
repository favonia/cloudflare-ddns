package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	markdownInlineLinkRE = regexp.MustCompile(`(?s)\[[^\]]+\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	markdownRefLinkRE    = regexp.MustCompile(`(?s)\[([^\]]+)\]\[([^\]]*)\]`)
	markdownRefDefRE     = regexp.MustCompile(`(?m)^\[([^\]]+)\]:\s*(\S+)`)
	htmlTargetRE         = regexp.MustCompile(`(?is)<(?:a|img|script|source)\b[^>]*\s(?:href|src)=["']([^"']+)["']`)
	htmlIDRE             = regexp.MustCompile(`(?is)\bid=["']([^"']+)["']`)
	urlRE                = regexp.MustCompile("https?://[^\\s<>)\"'`\\]\\\\]+")
)

type linkIssue struct {
	Kind   string
	Path   string
	Detail string
}

func (issue linkIssue) Render() string {
	return fmt.Sprintf("%s: %s: %s", issue.Kind, issue.Path, issue.Detail)
}

type linkSource struct {
	Path string
	Line int
}

func (source linkSource) Render() string {
	return fmt.Sprintf("%s:%d", source.Path, source.Line)
}

type externalLink struct {
	URL     string
	Sources []linkSource
}

type textTarget struct {
	Target string
	Offset int
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

func (patterns compiledPatterns) Match(candidate string) bool {
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

func filterFiles(files, includePatterns, excludePatterns []string) []string {
	include := newCompiledPatterns(includePatterns)
	exclude := newCompiledPatterns(excludePatterns)
	selected := make([]string, 0, len(files))
	for _, file := range files {
		if len(includePatterns) > 0 && !include.Match(file) {
			continue
		}
		if len(excludePatterns) > 0 && exclude.Match(file) {
			continue
		}
		selected = append(selected, file)
	}
	return selected
}

func validateLocalMarkdownLinks(files []string) []linkIssue {
	issues := make([]linkIssue, 0)
	idCache := map[string]map[string]bool{}
	for _, file := range files {
		text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			issues = append(issues, linkIssue{Kind: "read-error", Path: file, Detail: err.Error()})
			continue
		}
		for _, target := range collectMarkdownTargets(string(text)) {
			if isExternal(target.Target) || strings.HasPrefix(target.Target, "mailto:") {
				continue
			}
			targetPath, fragment := resolveLocalTarget(file, target.Target)
			if targetPath == "" {
				continue
			}
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(targetPath)))
			if err != nil || info == nil {
				issues = append(issues, linkIssue{
					Kind:   "broken-local-link",
					Path:   file,
					Detail: fmt.Sprintf("%s -> missing %s", target.Target, targetPath),
				})
				continue
			}
			if fragment != "" && isMarkdownFile(targetPath) {
				ids, ok := idCache[targetPath]
				if !ok {
					ids = markdownIDs(targetPath)
					idCache[targetPath] = ids
				}
				if !ids[fragment] {
					issues = append(issues, linkIssue{
						Kind:   "broken-anchor",
						Path:   file,
						Detail: fmt.Sprintf("%s -> missing #%s", target.Target, fragment),
					})
				}
			}
		}
	}
	return issues
}

func validateRepoPaths(files, trackedFiles, ignoredPaths []string) []linkIssue {
	ignored := map[string]bool{}
	for _, ignoredPath := range ignoredPaths {
		ignored[ignoredPath] = true
	}
	repoPathRE := newRepoPathRegexp(trackedFiles)
	if repoPathRE == nil {
		return nil
	}
	issues := make([]linkIssue, 0)
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			issues = append(issues, linkIssue{Kind: "read-error", Path: file, Detail: err.Error()})
			continue
		}
		for _, block := range extractCommentBlocks(file, string(data)) {
			for _, match := range repoPathRE.FindAllStringSubmatch(block, -1) {
				candidate := strings.TrimRight(match[2], ".,:;")
				if ignored[candidate] {
					continue
				}
				if _, err := statRepoPath(candidate); err != nil {
					issues = append(issues, linkIssue{
						Kind:   "broken-repo-path",
						Path:   file,
						Detail: candidate,
					})
				}
			}
		}
	}
	return issues
}

func newRepoPathRegexp(trackedFiles []string) *regexp.Regexp {
	prefixes := inferRepoPathPrefixes(trackedFiles)
	if len(prefixes) == 0 {
		return nil
	}
	quoted := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		quoted = append(quoted, regexp.QuoteMeta(prefix))
	}
	pattern := `(^|[^A-Za-z0-9_/:.-])((?:` + strings.Join(quoted, "|") + `)/(?:[A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+)`
	return regexp.MustCompile(pattern)
}

func inferRepoPathPrefixes(trackedFiles []string) []string {
	prefixes := map[string]bool{}
	for _, trackedFile := range trackedFiles {
		prefix, _, found := strings.Cut(trackedFile, "/")
		if !found || prefix == "" {
			continue
		}
		prefixes[prefix] = true
	}
	items := make([]string, 0, len(prefixes))
	for prefix := range prefixes {
		items = append(items, prefix)
	}
	slices.Sort(items)
	return items
}

func collectExternalURLs(files, ignoredURLs, ignoredPatterns []string) []externalLink {
	ignored := map[string]bool{}
	for _, ignoredURL := range ignoredURLs {
		ignored[ignoredURL] = true
	}
	ignoredURLPatterns := compileRegexps(ignoredPatterns)
	found := map[string]map[string]linkSource{}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			continue
		}
		text := string(data)
		if isMarkdownFile(file) {
			for _, target := range collectMarkdownTargets(text) {
				if !isExternal(target.Target) {
					continue
				}
				if ignored[target.Target] || anyRegexpMatch(ignoredURLPatterns, target.Target) {
					continue
				}
				recordExternalLink(found, target.Target, linkSource{
					Path: file,
					Line: lineNumberAtOffset(text, target.Offset),
				})
			}
		}
		for _, target := range collectTextURLs(text) {
			if ignored[target.Target] || anyRegexpMatch(ignoredURLPatterns, target.Target) {
				continue
			}
			recordExternalLink(found, target.Target, linkSource{
				Path: file,
				Line: lineNumberAtOffset(text, target.Offset),
			})
		}
	}
	urls := make([]externalLink, 0, len(found))
	for target, sourceMap := range found {
		sources := make([]linkSource, 0, len(sourceMap))
		for _, source := range sourceMap {
			sources = append(sources, source)
		}
		slices.SortFunc(sources, compareLinkSource)
		urls = append(urls, externalLink{
			URL:     target,
			Sources: sources,
		})
	}
	slices.SortFunc(urls, func(a, b externalLink) int {
		return strings.Compare(a.URL, b.URL)
	})
	return urls
}

func recordExternalLink(found map[string]map[string]linkSource, target string, source linkSource) {
	if found[target] == nil {
		found[target] = map[string]linkSource{}
	}
	found[target][source.Render()] = source
}

func compareLinkSource(a, b linkSource) int {
	if diff := strings.Compare(a.Path, b.Path); diff != 0 {
		return diff
	}
	return a.Line - b.Line
}

func lineNumberAtOffset(text string, offset int) int {
	return strings.Count(text[:offset], "\n") + 1
}

func collectTextURLs(text string) []textTarget {
	targets := make([]textTarget, 0, len(urlRE.FindAllStringIndex(text, -1)))
	for _, match := range urlRE.FindAllStringIndex(text, -1) {
		targets = append(targets, textTarget{
			Target: strings.TrimRight(text[match[0]:match[1]], ".,:;"),
			Offset: match[0],
		})
	}
	return targets
}

func collectMarkdownTargets(text string) []textTarget {
	targets := make([]textTarget, 0)
	for _, match := range markdownInlineLinkRE.FindAllStringSubmatchIndex(text, -1) {
		if match[0] > 0 && text[match[0]-1] == '!' {
			continue
		}
		targets = append(targets, textTarget{
			Target: text[match[2]:match[3]],
			Offset: match[2],
		})
	}
	refTargets := map[string]textTarget{}
	for _, match := range markdownRefDefRE.FindAllStringSubmatchIndex(text, -1) {
		label := strings.ToLower(strings.TrimSpace(text[match[2]:match[3]]))
		refTargets[label] = textTarget{
			Target: text[match[4]:match[5]],
			Offset: match[4],
		}
	}
	for _, match := range markdownRefLinkRE.FindAllStringSubmatchIndex(text, -1) {
		if match[0] > 0 && text[match[0]-1] == '!' {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(text[match[2]:match[3]]))
		ref := strings.ToLower(strings.TrimSpace(text[match[4]:match[5]]))
		if ref == "" {
			ref = label
		}
		target, ok := refTargets[ref]
		if ok {
			targets = append(targets, target)
		}
	}
	for _, match := range htmlTargetRE.FindAllStringSubmatchIndex(text, -1) {
		targets = append(targets, textTarget{
			Target: text[match[2]:match[3]],
			Offset: match[2],
		})
	}
	return targets
}

func markdownIDs(relativePath string) map[string]bool {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
	if err != nil {
		return nil
	}
	ids := map[string]bool{}
	// Trust only explicit HTML ids in tracked Markdown; renderer-generated heading ids are unstable.
	for _, match := range htmlIDRE.FindAllStringSubmatch(string(data), -1) {
		ids[match[1]] = true
	}
	return ids
}

func statRepoPath(relativePath string) (os.FileInfo, error) {
	cleaned := path.Clean(relativePath)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return nil, fmt.Errorf("invalid repo path %q", relativePath)
	}

	fullPath := filepath.Join(root, filepath.FromSlash(cleaned))
	relativeToRoot, err := filepath.Rel(root, fullPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo path %q relative to root: %w", relativePath, err)
	}
	relativeToRoot = filepath.ToSlash(relativeToRoot)
	if relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, "../") {
		return nil, fmt.Errorf("repo path escapes root %q", relativePath)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("stat repo path %q: %w", relativePath, err)
	}
	return info, nil
}

func resolveLocalTarget(sourceFile, target string) (string, string) {
	if target == "" || strings.HasPrefix(target, "mailto:") {
		return "", ""
	}
	pathPart, fragment, _ := strings.Cut(target, "#")
	var resolved string
	switch {
	case strings.HasPrefix(pathPart, "/"):
		resolved = strings.TrimLeft(pathPart, "/")
	case pathPart == "":
		resolved = sourceFile
	default:
		resolved = path.Clean(path.Join(path.Dir(sourceFile), pathPart))
	}
	return resolved, fragment
}

func extractCommentBlocks(file, text string) []string {
	switch path.Ext(file) {
	case ".go":
		return extractGoCommentBlocks(file, text)
	case ".yaml", ".yml":
		blocks := make([]string, 0)
		for line := range strings.SplitSeq(text, "\n") {
			trimmed := strings.TrimLeft(line, " \t")
			if strings.HasPrefix(trimmed, "#") {
				blocks = append(blocks, trimmed)
			}
		}
		return blocks
	default:
		return nil
	}
}

func extractGoCommentBlocks(file, text string) []string {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, file, text, parser.ParseComments|parser.AllErrors)
	if parsed != nil {
		blocks := make([]string, 0, len(parsed.Comments))
		for _, group := range parsed.Comments {
			blocks = append(blocks, group.Text())
		}
		return blocks
	}
	if err != nil {
		return nil
	}
	return nil
}

func isMarkdownFile(file string) bool {
	switch path.Ext(file) {
	case ".md", ".markdown":
		return true
	default:
		return false
	}
}

func isExternal(target string) bool {
	parsed, err := url.Parse(target)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func compileRegexps(patterns []string) []*regexp.Regexp {
	regexps := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		regexps = append(regexps, regexp.MustCompile(pattern))
	}
	return regexps
}

func anyRegexpMatch(regexps []*regexp.Regexp, candidate string) bool {
	for _, compiled := range regexps {
		if compiled.MatchString(candidate) {
			return true
		}
	}
	return false
}
