// Package main checks repository-local and external links for CI.
package main

//nolint:tagliatelle // underscore-separated config keys keep the config structure explicit
type config struct {
	LocalMarkdown localMarkdownConfig `json:"local_markdown"`
	RepoPaths     repoPathsConfig     `json:"repo_paths"`
	External      externalConfig      `json:"external"`
	ExternalProbe externalProbeConfig `json:"external_probe"`
}

// This tool checks for link degradation: whether a link that works today is
// likely to stop working later. Source files are selected before any extracted
// link targets are filtered, and each check owns its own source-file rules so
// the evaluation order stays local.
//
//nolint:tagliatelle // underscore-separated config keys keep the config structure explicit
type sourceFilesConfig struct {
	IncludeGlobs []string `json:"include_globs"`
	ExcludeGlobs []string `json:"exclude_globs"`
}

type localMarkdownConfig struct {
	SourceFiles sourceFilesConfig `json:"source_files"`
}

type repoPathsConfig struct {
	SourceFiles sourceFilesConfig `json:"source_files"`
	TargetPaths targetPathsConfig `json:"target_paths"`
}

//nolint:tagliatelle // underscore-separated config keys keep the config structure explicit
type targetPathsConfig struct {
	IgnoreExact []string `json:"ignore_exact"`
}

type externalConfig struct {
	SourceFiles sourceFilesConfig `json:"source_files"`
	TargetURLs  targetURLsConfig  `json:"target_urls"`
}

//nolint:tagliatelle // underscore-separated config keys keep the config structure explicit
type targetURLsConfig struct {
	IgnoreExact    []string `json:"ignore_exact"`
	IgnorePatterns []string `json:"ignore_patterns"`
}

//nolint:tagliatelle // underscore-separated config keys match the prior JSON config shape
type externalProbeConfig struct {
	TimeoutSeconds          float64  `json:"timeout_seconds"`
	Retries                 int      `json:"retries"`
	MaxWorkers              int      `json:"max_workers"`
	UserAgent               string   `json:"user_agent"`
	NetworkErrorsAreWarning bool     `json:"network_errors_are_warnings"`
	WarningStatuses         []int    `json:"warning_statuses"`
	WarningURLPatterns      []string `json:"warning_url_patterns"`
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
