package external

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
)

var (
	redirectStatusCodes = map[int]bool{
		http.StatusMovedPermanently:  true,
		http.StatusFound:             true,
		http.StatusSeeOther:          true,
		http.StatusTemporaryRedirect: true,
		http.StatusPermanentRedirect: true,
	}
	headFallbackStatuses = map[int]bool{
		http.StatusForbidden:        true,
		http.StatusMethodNotAllowed: true,
		http.StatusTooManyRequests:  true,
		http.StatusNotImplemented:   true,
	}
	maxRedirectHops = 10
)

type probeResult struct {
	URL     string
	Status  int
	Err     string
	Hops    []probeHop
	Sources []extract.LinkSource
}

type probeHop struct {
	URL    string
	Status int
}

// probeURL runs the configured HEAD/GET probe cycle with retries for one URL.
func probeURL(link extract.ExternalLink, timeout time.Duration, retries int, userAgent string) probeResult {
	var last probeResult
	for attempt := 0; attempt <= retries; attempt++ {
		for _, method := range []string{http.MethodHead, http.MethodGet} {
			result := fetchURL(link.URL, method, timeout, userAgent)
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

// fetchURL performs one probe attempt while recording the redirect chain
// without following redirects automatically.
func fetchURL(target, method string, timeout time.Duration, userAgent string) probeResult {
	client := &http.Client{
		Timeout: timeout,
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
				currentMethod = http.MethodHead
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

// classifyRequestError unwraps url.Error so operator diagnostics focus on the
// transport failure that actually mattered.
func classifyRequestError(err error) string {
	var urlError *url.Error
	if errors.As(err, &urlError) {
		if urlError.Err != nil {
			return urlError.Err.Error()
		}
	}
	return err.Error()
}
