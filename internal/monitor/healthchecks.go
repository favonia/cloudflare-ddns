package monitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type HealthChecks struct {
	BaseURL         string
	RedactedBaseURL string
	Timeout         time.Duration
	MaxRetries      int
}

const (
	HealthChecksDefaultTimeout    = 10 * time.Second
	HealthChecksDefaultMaxRetries = 5
)

type HealthChecksOption func(*HealthChecks)

func SetHealthChecksMaxRetries(maxRetries int) HealthChecksOption {
	if maxRetries <= 0 {
		panic("maxRetries <= 0")
	}
	return func(h *HealthChecks) {
		h.MaxRetries = maxRetries
	}
}

func NewHealthChecks(ppfmt pp.PP, rawURL string, os ...HealthChecksOption) (Monitor, bool) {
	url, err := url.Parse(rawURL)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse the Healthchecks URL %q: %v", rawURL, err)
		return nil, false
	}

	if !(url.IsAbs() && url.Opaque == "" && url.Host != "" && url.Fragment == "" && url.ForceQuery == false && url.RawQuery == "") { //nolint: lll
		ppfmt.Errorf(pp.EmojiUserError, `The URL %q does not look like a valid Healthchecks URL.`, url.Redacted())
		ppfmt.Errorf(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc".`)
		return nil, false
	}

	h := &HealthChecks{
		BaseURL:         url.String(),
		RedactedBaseURL: url.Redacted(),
		Timeout:         HealthChecksDefaultTimeout,
		MaxRetries:      HealthChecksDefaultMaxRetries,
	}

	for _, o := range os {
		o(h)
	}

	return h, true
}

func (h *HealthChecks) DescribeService() string {
	return "Healthchecks.io"
}

func (h *HealthChecks) DescribeBaseURL() string {
	return h.RedactedBaseURL
}

//nolint: funlen
func (h *HealthChecks) ping(ctx context.Context, ppfmt pp.PP, url string, redatedURL string) bool {
	for retries := 0; retries < h.MaxRetries; retries++ {
		if retries > 0 {
			time.Sleep(time.Second << (retries - 1))
		}

		ctx, cancel := context.WithTimeout(ctx, h.Timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			ppfmt.Warningf(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v", redatedURL, err)
			return false
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			ppfmt.Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", redatedURL, err)
			ppfmt.Infof(pp.EmojiRepeatOnce, "Trying again . . .")
			continue
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			ppfmt.Warningf(pp.EmojiError, "Failed to read HTTP(S) response from %q: %v", redatedURL, err)
			ppfmt.Infof(pp.EmojiRepeatOnce, "Trying again . . .")
			continue
		}

		/*
			Code and body for uuid API:

			200 OK
			200 OK (not found)
			200 OK (rate limited)
			400 invalid url format

			Code and body for the slug API:

			200 OK
			400 invalid url format
			404 not found
			409 ambiguous slug
			429 rate limit exceeded
		*/

		bodyAsString := strings.TrimSpace(string(body))
		if bodyAsString != "OK" {
			ppfmt.Warningf(
				pp.EmojiError,
				"Failed to ping %q; got response code: %d %s",
				redatedURL,
				resp.StatusCode,
				bodyAsString,
			)
			return false
		}

		ppfmt.Infof(pp.EmojiNotification, "Successfully pinged %q.", redatedURL)
		return true
	}

	ppfmt.Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q in %d time(s).", redatedURL, h.MaxRetries)
	return false
}

func (h *HealthChecks) Success(ctx context.Context, ppfmt pp.PP) bool {
	return h.ping(ctx, ppfmt, h.BaseURL, h.RedactedBaseURL)
}

func (h *HealthChecks) Start(ctx context.Context, ppfmt pp.PP) bool {
	return h.ping(ctx, ppfmt, h.BaseURL+"/start", h.RedactedBaseURL+"/start")
}

func (h *HealthChecks) Failure(ctx context.Context, ppfmt pp.PP) bool {
	return h.ping(ctx, ppfmt, h.BaseURL+"/fail", h.RedactedBaseURL+"/fail")
}

func (h *HealthChecks) ExitStatus(ctx context.Context, ppfmt pp.PP, code int) bool {
	if code < 0 || code > 255 {
		ppfmt.Errorf(pp.EmojiImpossible, "Exit code (%i) not within the range 0-255.", code)
		return false
	}

	return h.ping(ctx, ppfmt, fmt.Sprintf("%s/%d", h.BaseURL, code), fmt.Sprintf("%s/%d", h.RedactedBaseURL, code))
}
