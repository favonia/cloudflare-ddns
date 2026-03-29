package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

var (
	redirectStatusCodes  = map[int]bool{301: true, 302: true, 303: true, 307: true, 308: true}
	headFallbackStatuses = map[int]bool{403: true, 405: true, 429: true, 501: true}
	maxRedirectHops      = 10
)

type probeResult struct {
	URL     string
	Status  int
	Err     string
	Hops    []probeHop
	Sources []linkSource
}

type probeHop struct {
	URL    string
	Status int
}

func runExternalProbe(urls []externalLink, cfg externalProbeConfig) ([]probeResult, []probeResult) {
	warningStatuses := map[int]bool{}
	for _, status := range cfg.WarningStatuses {
		warningStatuses[status] = true
	}
	warningPatterns := compileRegexps(cfg.WarningURLPatterns)
	workerCount := max(cfg.MaxWorkers, 1)

	jobs := make(chan externalLink)
	results := make(chan probeResult)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Go(func() {
			for target := range jobs {
				results <- probeExternalURL(target, cfg.TimeoutSeconds, cfg.Retries, cfg.UserAgent)
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

	failures := make([]probeResult, 0)
	warnings := make([]probeResult, 0)
	for result := range results {
		if shouldSuppressExternalWarning(result) {
			continue
		}
		softWarning := anyRegexpMatch(warningPatterns, result.URL)
		switch {
		case softWarning || (cfg.NetworkErrorsAreWarning && result.Status == 0):
			warnings = append(warnings, result)
		case result.Status == 0:
			failures = append(failures, result)
		case result.Status >= 400 && !warningStatuses[result.Status]:
			failures = append(failures, result)
		case warningStatuses[result.Status] || len(result.Hops) > 1:
			warnings = append(warnings, result)
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

func writeExternalFindings(stderr io.Writer, failures, warnings []probeResult) bool {
	for _, warning := range warnings {
		_, _ = fmt.Fprintln(stderr, formatExternalResult("warning", warning))
	}
	for _, failure := range failures {
		_, _ = fmt.Fprintln(stderr, formatExternalResult("failure", failure))
	}
	return len(failures) > 0
}

func probeExternalURL(link externalLink, timeoutSeconds float64, retries int, userAgent string) probeResult {
	var last probeResult
	for attempt := 0; attempt <= retries; attempt++ {
		for _, method := range []string{http.MethodHead, http.MethodGet} {
			result := fetchURL(link.URL, method, timeoutSeconds, userAgent)
			result.Sources = link.Sources
			last = result
			if method == http.MethodHead && headFallbackStatuses[result.Status] {
				continue
			}
			if result.Status != 0 || result.Err != "" {
				return result
			}
		}
	}
	return last
}

func fetchURL(target, method string, timeoutSeconds float64, userAgent string) probeResult {
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds * float64(time.Second)),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	currentURL := target
	currentMethod := method
	hops := make([]probeHop, 0, maxRedirectHops+1)

	for range maxRedirectHops {
		request, err := http.NewRequestWithContext(context.Background(), currentMethod, currentURL, nil)
		if err != nil {
			return probeResult{URL: target, Err: err.Error(), Hops: append(slices.Clone(hops), probeHop{URL: currentURL})}
		}
		request.Header.Set("User-Agent", userAgent)

		response, err := client.Do(request)
		if err != nil {
			return probeResult{
				URL:  target,
				Err:  classifyRequestError(err),
				Hops: append(slices.Clone(hops), probeHop{URL: currentURL}),
			}
		}
		hops = append(hops, probeHop{
			URL:    currentURL,
			Status: response.StatusCode,
		})
		if _, copyErr := io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20)); copyErr != nil {
			_ = response.Body.Close()
			return probeResult{URL: target, Status: response.StatusCode, Err: copyErr.Error(), Hops: hops}
		}
		if closeErr := response.Body.Close(); closeErr != nil {
			return probeResult{URL: target, Status: response.StatusCode, Err: closeErr.Error(), Hops: hops}
		}

		if redirectStatusCodes[response.StatusCode] {
			location, err := response.Location()
			if err != nil {
				return probeResult{URL: target, Status: response.StatusCode, Err: err.Error(), Hops: hops}
			}
			currentURL = location.String()
			if response.StatusCode == http.StatusSeeOther {
				currentMethod = http.MethodGet
			}
			continue
		}

		return probeResult{
			URL:    target,
			Status: response.StatusCode,
			Hops:   hops,
		}
	}

	return probeResult{URL: target, Err: "too many redirects", Hops: hops}
}

func classifyRequestError(err error) string {
	var urlError *url.Error
	if errors.As(err, &urlError) {
		if urlError.Err != nil {
			return urlError.Err.Error()
		}
	}
	return err.Error()
}

func formatExternalResult(prefix string, result probeResult) string {
	locationText := formatLinkSources(result.Sources)
	if result.Err != "" {
		return fmt.Sprintf("%s: network error: %s [%s]", prefix, formatProbeChain(result), locationText)
	}
	return fmt.Sprintf("%s: %s [%s]", prefix, formatProbeChain(result), locationText)
}

func formatLinkSources(sources []linkSource) string {
	if len(sources) == 0 {
		return "source unknown"
	}
	rendered := make([]string, 0, len(sources))
	for _, source := range sources {
		rendered = append(rendered, source.Render())
	}
	return strings.Join(rendered, ", ")
}

func formatProbeChain(result probeResult) string {
	if len(result.Hops) == 0 {
		if result.Err != "" {
			return fmt.Sprintf("%s (network error: %s)", result.URL, result.Err)
		}
		return fmt.Sprintf("%s (HTTP %d)", result.URL, result.Status)
	}
	parts := make([]string, 0, len(result.Hops))
	lastIndex := len(result.Hops) - 1
	for index, hop := range result.Hops {
		if result.Err != "" && index == lastIndex && hop.Status == 0 {
			parts = append(parts, fmt.Sprintf("%s (network error: %s)", hop.URL, result.Err))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%d)", hop.URL, hop.Status))
	}
	return strings.Join(parts, " -> ")
}
