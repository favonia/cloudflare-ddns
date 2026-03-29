// Package local validates repository-local links and repo-path references.
package local

import "github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/scope"

type targetPathsConfig struct {
	// IgnoreExact suppresses exact extracted repository-relative paths from the
	// broken-path check.
	IgnoreExact []string
}

type markdownLinksConfig struct {
	// SourceFiles selects Markdown files whose local links and anchors should be
	// validated.
	SourceFiles scope.SourceFilesConfig
}

type commentRepoPathsConfig struct {
	// SourceFiles selects non-Markdown files whose comment blocks should be
	// scanned for repository-relative paths.
	SourceFiles scope.SourceFilesConfig
	// TargetPaths configures which extracted repository-relative paths should be
	// ignored after scanning.
	TargetPaths targetPathsConfig
}

type config struct {
	// MarkdownLinks configures Markdown-to-repository checks such as missing
	// files and broken Markdown anchors.
	MarkdownLinks markdownLinksConfig
	// CommentRepoPaths configures repository-path checks for paths extracted
	// from comment text in non-Markdown source files.
	CommentRepoPaths commentRepoPathsConfig
}

func defaultConfig() config {
	return config{
		MarkdownLinks: markdownLinksConfig{
			SourceFiles: scope.SourceFilesConfig{
				IncludeGlobs: []string{
					"*.md",
					"*.markdown",
					"**/*.md",
					"**/*.markdown",
				},
				ExcludeGlobs: []string{},
			},
		},
		CommentRepoPaths: commentRepoPathsConfig{
			SourceFiles: scope.SourceFilesConfig{
				IncludeGlobs: []string{
					"*.go",
					"*.yaml",
					"*.yml",
					"**/*.go",
					"**/*.yaml",
					"**/*.yml",
				},
				ExcludeGlobs: []string{
					"scripts/github-actions/cloudflare-doc-watch/cases.go",
				},
			},
			TargetPaths: targetPathsConfig{
				IgnoreExact: []string{
					"scripts/github-actions/link-check/internal/local/config.go",
				},
			},
		},
	}
}
