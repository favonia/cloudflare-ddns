// Package main checks repository-local and external links for CI.
package main

type config struct {
	LocalMarkdown localMarkdownConfig
	RepoPaths     repoPathsConfig
	External      externalConfig
	ExternalProbe externalProbeConfig
}

// This tool checks for link degradation: whether a link that works today is
// likely to stop working later. Source files are selected before any extracted
// link targets are filtered, and each check owns its own source-file rules so
// the evaluation order stays local.
type sourceFilesConfig struct {
	IncludeGlobs []string
	ExcludeGlobs []string
}

type localMarkdownConfig struct {
	SourceFiles sourceFilesConfig
}

type repoPathsConfig struct {
	SourceFiles sourceFilesConfig
	TargetPaths targetPathsConfig
}

type targetPathsConfig struct {
	IgnoreExact []string
}

type externalConfig struct {
	SourceFiles sourceFilesConfig
	TargetURLs  targetURLsConfig
}

type targetURLsConfig struct {
	IgnoreExact    []string
	IgnorePatterns []string
}

type externalProbeConfig struct {
	TimeoutSeconds          float64
	Retries                 int
	MaxWorkers              int
	UserAgent               string
	NetworkErrorsAreWarning bool
	WarningStatuses         []int
	WarningURLPatterns      []string
}

func defaultConfig() config {
	return config{
		LocalMarkdown: localMarkdownConfig{
			SourceFiles: sourceFilesConfig{
				IncludeGlobs: []string{
					"*.md",
					"*.markdown",
					"**/*.md",
					"**/*.markdown",
				},
				ExcludeGlobs: []string{
					"docs/private/**",
				},
			},
		},
		RepoPaths: repoPathsConfig{
			SourceFiles: sourceFilesConfig{
				IncludeGlobs: []string{
					"*.go",
					"*.yaml",
					"*.yml",
					"**/*.go",
					"**/*.yaml",
					"**/*.yml",
				},
				ExcludeGlobs: []string{
					"docs/private/**",
					"scripts/github-actions/cloudflare-doc-watch/cases.go",
				},
			},
			TargetPaths: targetPathsConfig{
				IgnoreExact: []string{
					"scripts/github-actions/link-check/config.go",
				},
			},
		},
		External: externalConfig{
			SourceFiles: sourceFilesConfig{
				IncludeGlobs: []string{
					".github/workflows/*.yml",
					".github/workflows/*.yaml",
					"*.go",
					"*.json",
					"*.yaml",
					"*.yml",
					"*.md",
					"*.markdown",
					"**/*.go",
					"**/*.json",
					"**/*.yaml",
					"**/*.yml",
					"**/*.md",
					"**/*.markdown",
				},
				ExcludeGlobs: []string{
					"docs/private/**",
					"**/*_test.go",
					"scripts/github-actions/link-check/config.go",
				},
			},
			TargetURLs: targetURLsConfig{
				IgnoreExact: []string{
					"https://api.github.com",
					// These endpoints are operational probes or auth-only APIs rather than
					// durable documentation targets, so probing them adds noise.
					"https://one.one.one.one/cdn-cgi/trace",
					"https://api.cloudflare.com/cdn-cgi/trace",
					"https://api.cloudflare.com/client/v4/user/tokens/verify",
					"https://token.actions.githubusercontent.com",
					"https://cloudflare-dns.com/dns-query",
					"https://api4.ipify.org",
					"https://api6.ipify.org",
				},
				IgnorePatterns: []string{
					"^https?:///",
					"^https?://(?:0\\.0\\.0\\.0|1\\.2\\.3\\.4)(?:[/:?]|$)",
					"^https?://[^/\\s]*example(?:\\.com)?(?:[/:?]|$)",
					// Same-repo GitHub issue URLs are stable identifiers, so probing them adds
					// rate-limit noise rather than meaningful link-rot coverage.
					"^https://github\\.com/favonia/cloudflare-ddns/issues/[0-9]+$",
					"^https://(?:healthchecks|uptime)\\.example(?:[/:?]|$)",
					"^https://hc-ping\\.com/01234567-0123-0123-0123-0123456789abc$",
					"^https://localhost(?:[/:?]|$)",
					"^https://some\\.host\\.name(?:[/:?]|$)",
					"^https://user:pass@host(?:[/:?]|$)",
					"%s",
				},
			},
		},
		ExternalProbe: externalProbeConfig{
			TimeoutSeconds:          10,
			Retries:                 1,
			MaxWorkers:              8,
			UserAgent:               "cloudflare-ddns-link-check/1.0",
			NetworkErrorsAreWarning: false,
			WarningStatuses: []int{
				403,
				429,
			},
			WarningURLPatterns: []string{},
		},
	}
}
