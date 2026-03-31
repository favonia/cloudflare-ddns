package external

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/scope"
)

// Run executes external-link collection and probing under the provided
// repository root.
func Run(root string, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("link-check external", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: link-check external")
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

	cfg := defaultConfig()
	externalFiles := scope.FilterFiles(
		trackedFiles,
		cfg.Links.SourceFiles.IncludeGlobs,
		cfg.Links.SourceFiles.ExcludeGlobs,
	)
	externalURLs := collectURLs(
		root,
		externalFiles,
		cfg.Links.TargetURLs.IgnoreExact,
		cfg.Links.TargetURLs.IgnorePatterns,
	)
	_, _ = fmt.Fprintf(stdout, "Collected %d external URLs.\n", len(externalURLs))
	_, _ = fmt.Fprintf(stdout, "Probing %d URLs across %d hosts (max %d per host, %v delay)...\n",
		len(externalURLs), countUniqueHosts(externalURLs), cfg.Probe.MaxPerHost, cfg.Probe.PerHostDelay)
	failures, warnings := runProbe(externalURLs, cfg.Probe)
	if writeFindings(stderr, failures, warnings) {
		return 1
	}

	_, _ = fmt.Fprintln(stdout, "External link probes passed.")
	return 0
}

// collectURLs extracts unique external URLs plus their source locations from
// the selected tracked files.
func collectURLs(root string, files, ignoredURLs, ignoredPatterns []string) []extract.ExternalLink {
	ignored := map[string]bool{}
	for _, ignoredURL := range ignoredURLs {
		ignored[ignoredURL] = true
	}
	ignoredURLPatterns := compileRegexps(ignoredPatterns)
	found := map[string]map[string]extract.LinkSource{}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			continue
		}
		for _, target := range extract.Extract(file, string(data)).HTTPLinks {
			if ignored[target.URL] || anyRegexpMatch(ignoredURLPatterns, target.URL) {
				continue
			}
			recordLink(found, target.URL, extract.LinkSource{
				Path: file,
				Line: target.Line,
			})
		}
	}
	urls := make([]extract.ExternalLink, 0, len(found))
	for target, sourceMap := range found {
		sources := make([]extract.LinkSource, 0, len(sourceMap))
		for _, source := range sourceMap {
			sources = append(sources, source)
		}
		slices.SortFunc(sources, func(a, b extract.LinkSource) int {
			return a.Compare(b)
		})
		urls = append(urls, extract.ExternalLink{
			URL:     target,
			Sources: sources,
		})
	}
	slices.SortFunc(urls, func(a, b extract.ExternalLink) int {
		return strings.Compare(a.URL, b.URL)
	})
	return urls
}

// recordLink deduplicates repeated occurrences of one URL by rendered source
// location while preserving stable source records.
func recordLink(found map[string]map[string]extract.LinkSource, target string, source extract.LinkSource) {
	if found[target] == nil {
		found[target] = map[string]extract.LinkSource{}
	}
	found[target][source.Render()] = source
}
