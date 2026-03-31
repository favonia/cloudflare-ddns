package external

import (
	"net/http"
	"time"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/scope"
)

type targetURLsConfig struct {
	// IgnoreExact suppresses exact extracted external URLs from probing.
	IgnoreExact []string
	// IgnorePatterns suppresses extracted external URLs whose full string
	// matches one of these regular expressions.
	IgnorePatterns []string
}

type linksConfig struct {
	// SourceFiles selects files whose external URLs should be collected.
	SourceFiles scope.SourceFilesConfig
	// TargetURLs configures which extracted external URLs should be ignored
	// before probing.
	TargetURLs targetURLsConfig
}

type probeConfig struct {
	// Timeout is the per-request timeout for each HEAD or GET probe.
	Timeout time.Duration
	// Retries is the number of additional probe attempts after the first probe
	// cycle returns only network errors.
	Retries int
	// MaxWorkers bounds concurrent external probes; values below 1 still run
	// with one worker.
	MaxWorkers int
	// MaxPerHost bounds concurrent probes to the same host; values below 1
	// still allow one concurrent probe per host.
	MaxPerHost int
	// PerHostDelay is the minimum delay between starting consecutive probes
	// to the same host.
	PerHostDelay time.Duration
	// UserAgent is sent with every outbound probe request.
	UserAgent string
	// NetworkErrorsAreWarning downgrades final network errors from failures to
	// warnings.
	NetworkErrorsAreWarning bool
	// WarningStatuses downgrades these final HTTP response statuses from
	// failures to warnings.
	WarningStatuses []int
	// WarningURLPatterns downgrades matching final URL results to warnings
	// regardless of HTTP status.
	WarningURLPatterns []string
}

type config struct {
	// Links configures external-URL collection before live probing.
	Links linksConfig
	// Probe configures how collected external URLs are probed and which probe
	// outcomes are downgraded to warnings.
	Probe probeConfig
}

func defaultConfig() config {
	return config{
		Links: linksConfig{
			SourceFiles: scope.SourceFilesConfig{
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
					"**/*_test.go",
					"scripts/github-actions/link-check/internal/external/config.go",
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
					"^https://(?:healthchecks|uptime)\\.example(?:[/:?]|$)",
					"^https://hc-ping\\.com/01234567-0123-0123-0123-0123456789abc$",
					"^https://localhost(?:[/:?]|$)",
					"^https://some\\.host\\.name(?:[/:?]|$)",
					"^https://user:pass@host(?:[/:?]|$)",
					"%s",
				},
			},
		},
		Probe: probeConfig{
			Timeout:                 10 * time.Second,
			Retries:                 1,
			MaxWorkers:              8,
			MaxPerHost:              2,
			PerHostDelay:            500 * time.Millisecond,
			UserAgent:               "cloudflare-ddns-link-check/1.0",
			NetworkErrorsAreWarning: false,
			WarningStatuses: []int{
				http.StatusForbidden,
				http.StatusTooManyRequests,
			},
			WarningURLPatterns: []string{},
		},
	}
}
