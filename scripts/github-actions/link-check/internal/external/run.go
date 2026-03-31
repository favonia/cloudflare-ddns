package external

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/scope"
)

// Run executes external-link collection and probing under the provided
// repository root.
func Run(root string, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("link-check external", flag.ContinueOnError)
	flags.SetOutput(stderr)
	runPattern := flags.String("run", "", "probe only URLs matching this regexp (like go test -run)")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: link-check external [-run pattern]")
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

	var runFilter *regexp.Regexp
	if *runPattern != "" {
		var err error
		runFilter, err = regexp.Compile(*runPattern)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "invalid -run pattern: %v\n", err)
			return 1
		}
	}

	trackedFiles, err := scope.TrackedFiles(root)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "Repo root: %s\n", root)
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
	if runFilter != nil {
		filtered := externalURLs[:0]
		for _, link := range externalURLs {
			if runFilter.MatchString(link.URL) {
				filtered = append(filtered, link)
			}
		}
		externalURLs = filtered
		_, _ = fmt.Fprintf(stdout, "Filtered to %d URLs matching %q.\n", len(externalURLs), runFilter.String())
	}
	_, _ = fmt.Fprintf(stdout, "Probing %d URLs across %d hosts (max %d per host, %v delay)...\n",
		len(externalURLs), countUniqueHosts(externalURLs), cfg.Probe.MaxPerHost, cfg.Probe.PerHostDelay)
	failures, warnings := runProbe(externalURLs, cfg.Probe, stdout)

	writeFindings(stderr, failures, warnings)
	writeFindingsForGithub(stdout, failures, warnings)

	if len(failures) > 0 {
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

// runProbe probes the collected external URLs and classifies each outcome as a
// failure or warning according to probe policy.
func runProbe(urls []extract.ExternalLink, cfg probeConfig, stdout io.Writer) ([]probeResult, []probeResult) {
	warningStatuses := map[int]bool{}
	for _, status := range cfg.WarningStatuses {
		warningStatuses[status] = true
	}
	warningPatterns := compileRegexps(cfg.WarningURLPatterns)
	workerCount := max(cfg.MaxWorkers, 1)
	throttle := newHostThrottle(urls, cfg.MaxPerHost, cfg.PerHostDelay)

	jobs := make(chan extract.ExternalLink)
	results := make(chan probeResult)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Go(func() {
			for target := range jobs {
				host := hostFromURL(target.URL)
				throttle.acquire(host)
				result := probeURL(target, cfg.Timeout, cfg.Retries, cfg.UserAgent)
				throttle.release(host)
				results <- result
			}
		})
	}

	go func() {
		for _, target := range urls {
			jobs <- target
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	total := len(urls)
	completed := 0
	width := len(fmt.Sprint(total))
	failures := make([]probeResult, 0)
	warnings := make([]probeResult, 0)
	for result := range results {
		completed++
		progress := fmt.Sprintf("[%*d/%d]", width, completed, total)
		if shouldSuppressWarning(result) {
			_, _ = fmt.Fprintf(stdout, "%s ok: %s\n", progress, formatProbeChain(result))
			continue
		}
		softWarning := anyRegexpMatch(warningPatterns, result.URL)
		switch {
		case softWarning || (cfg.NetworkErrorsAreWarning && result.Status == 0):
			_, _ = fmt.Fprintf(stdout, "%s warning: %s\n", progress, formatResult(result))
			warnings = append(warnings, result)
		case result.Status == 0:
			_, _ = fmt.Fprintf(stdout, "%s FAILURE: %s\n", progress, formatResult(result))
			failures = append(failures, result)
		case result.Status >= 400 && !warningStatuses[result.Status]:
			_, _ = fmt.Fprintf(stdout, "%s FAILURE: %s\n", progress, formatResult(result))
			failures = append(failures, result)
		case warningStatuses[result.Status] || len(result.Hops) > 1:
			_, _ = fmt.Fprintf(stdout, "%s warning: %s\n", progress, formatResult(result))
			warnings = append(warnings, result)
		default:
			_, _ = fmt.Fprintf(stdout, "%s ok: %s\n", progress, formatProbeChain(result))
		}
	}

	slices.SortFunc(failures, func(a, b probeResult) int {
		return strings.Compare(a.URL, b.URL)
	})
	slices.SortFunc(warnings, func(a, b probeResult) int {
		return strings.Compare(a.URL, b.URL)
	})
	return failures, warnings
}
