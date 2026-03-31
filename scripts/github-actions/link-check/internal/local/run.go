package local

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/scope"
)

// Run executes repository-local link checks under the provided repository
// root.
func Run(root string, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("link-check local", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: link-check local")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 1
	}

	trackedFiles, err := scope.TrackedFiles(root)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "Repo root: %s\n", root)
	cfg := defaultConfig()
	markdownFiles := scope.FilterFiles(
		trackedFiles,
		cfg.MarkdownLinks.SourceFiles.IncludeGlobs,
		cfg.MarkdownLinks.SourceFiles.ExcludeGlobs,
	)
	textFiles := scope.FilterFiles(
		trackedFiles,
		cfg.CommentRepoPaths.SourceFiles.IncludeGlobs,
		cfg.CommentRepoPaths.SourceFiles.ExcludeGlobs,
	)

	localIssues := validateLocalMarkdownLinks(root, markdownFiles)
	localIssues = append(
		localIssues,
		validateCommentRepoPaths(root, textFiles, trackedFiles, cfg.CommentRepoPaths.TargetPaths.IgnoreExact)...,
	)
	if len(localIssues) == 0 {
		_, _ = fmt.Fprintln(stdout, "Local link checks passed.")
		return 0
	}

	slices.SortFunc(localIssues, func(a, b extract.Issue) int {
		return strings.Compare(a.Render(), b.Render())
	})
	for _, issue := range localIssues {
		_, _ = fmt.Fprintln(stderr, issue.Render())
	}
	writeIssuesForGithub(stdout, localIssues)
	return 1
}

// validateLocalMarkdownLinks checks Markdown local-link targets and explicit
// Markdown anchors.
func validateLocalMarkdownLinks(root string, files []string) []extract.Issue {
	issues := make([]extract.Issue, 0)
	targetInfoCache := map[string]extract.LocalTargetInfo{}
	for _, file := range files {
		text, err := readRepoFile(root, file)
		if err != nil {
			issues = append(issues, extract.Issue{Kind: "read-error", Path: file, Detail: err.Error()})
			continue
		}
		links := extract.Extract(file, string(text))
		for _, target := range links.LocalReferences {
			targetPath, fragment := resolveLocalTarget(file, target.Target)
			if targetPath == "" {
				continue
			}
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(targetPath)))
			if err != nil || info == nil {
				issues = append(issues, extract.Issue{
					Kind:   "broken-local-link",
					Path:   file,
					Detail: fmt.Sprintf("%s -> missing %s", target.Target, targetPath),
				})
				continue
			}
			if fragment != "" {
				targetInfo, ok := targetInfoCache[targetPath]
				if !ok {
					targetText, err := readRepoFile(root, targetPath)
					if err != nil {
						issues = append(issues, extract.Issue{
							Kind:   "read-error",
							Path:   targetPath,
							Detail: err.Error(),
						})
						continue
					}
					targetInfo = extract.LocalTargets(targetPath, string(targetText))
					targetInfoCache[targetPath] = targetInfo
				}
				if targetInfo.SupportsFragments && !targetInfo.Fragments[fragment] {
					issues = append(issues, extract.Issue{
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

// resolveLocalTarget resolves one Markdown local-link target relative to its
// source file and returns the repository-relative path plus fragment.
func resolveLocalTarget(sourceFile, target string) (string, string) {
	if target == "" {
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

// newRepoPathRegexp builds the repository-path detector used for comment-text
// path validation. The expression is derived from tracked top-level prefixes so
// it only matches paths that plausibly belong to this repository.
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

// inferRepoPathPrefixes derives the allowed leading path segments from tracked
// files so repo-path detection can stay repository-specific.
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

// statRepoPath validates that a repository-relative path stays under root and
// then stats the resolved filesystem path.
func statRepoPath(root, relativePath string) (os.FileInfo, error) {
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

// validateCommentRepoPaths scans comment text for repository-relative paths and
// reports paths that do not resolve under the repository root.
func validateCommentRepoPaths(root string, files, trackedFiles, ignoredPaths []string) []extract.Issue {
	ignored := map[string]bool{}
	for _, ignoredPath := range ignoredPaths {
		ignored[ignoredPath] = true
	}
	repoPathRE := newRepoPathRegexp(trackedFiles)
	if repoPathRE == nil {
		return nil
	}
	issues := make([]extract.Issue, 0)
	for _, file := range files {
		data, err := readRepoFile(root, file)
		if err != nil {
			issues = append(issues, extract.Issue{Kind: "read-error", Path: file, Detail: err.Error()})
			continue
		}
		for _, block := range extract.Extract(file, string(data)).CommentBlocks {
			for _, match := range repoPathRE.FindAllStringSubmatch(block, -1) {
				candidate := strings.TrimRight(match[2], ".,:;")
				if ignored[candidate] {
					continue
				}
				if _, err := statRepoPath(root, candidate); err != nil {
					issues = append(issues, extract.Issue{
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

func readRepoFile(root, relativePath string) ([]byte, error) {
	info, err := statRepoPath(root, relativePath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("read repo path %q: is a directory", relativePath)
	}

	fullPath := filepath.Join(root, filepath.FromSlash(path.Clean(relativePath)))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read repo path %q: %w", relativePath, err)
	}
	return data, nil
}
