package monitor

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
	HeathChecksDefaultTimeout     = 10 * time.Second
	HealthChecksDefaultMaxRetries = 5
)

func NewHealthChecks(ppfmt pp.PP, rawURL string) (Monitor, bool) {
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

	return &HealthChecks{
		BaseURL:         url.String(),
		RedactedBaseURL: url.Redacted(),
		Timeout:         HeathChecksDefaultTimeout,
		MaxRetries:      HealthChecksDefaultMaxRetries,
	}, true
}

func (h *HealthChecks) DescribeService() string {
	return "Healthchecks.io"
}

func (h *HealthChecks) DescribeBaseURL() string {
	return h.RedactedBaseURL
}

func (h *HealthChecks) reallyPing(ctx context.Context, ppfmt pp.PP, url string, redatedURL string) bool {
	for retries := 0; retries < h.MaxRetries; retries++ {
		ctx, cancel := context.WithTimeout(ctx, h.Timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
		if err != nil {
			ppfmt.Warningf(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v", redatedURL, err)
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			ppfmt.Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", redatedURL, err)
			continue
		}

		resp.Body.Close()
		return true
	}

	ppfmt.Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q within %d time(s).", redatedURL, h.MaxRetries)
	return false
}

func (h *HealthChecks) Success(ctx context.Context, ppfmt pp.PP) bool {
	return h.reallyPing(ctx, ppfmt, h.BaseURL, h.RedactedBaseURL)
}

func (h *HealthChecks) Start(ctx context.Context, ppfmt pp.PP) bool {
	return h.reallyPing(ctx, ppfmt, h.BaseURL+"/start", h.RedactedBaseURL+"/start")
}

func (h *HealthChecks) Failure(ctx context.Context, ppfmt pp.PP) bool {
	return h.reallyPing(ctx, ppfmt, h.BaseURL+"/fail", h.RedactedBaseURL+"/fail")
}

func (h *HealthChecks) ExitStatus(ctx context.Context, ppfmt pp.PP, code int) bool {
	if code < 0 || code > 255 {
		ppfmt.Errorf(pp.EmojiImpossible, "Exit code (%i) not within the range 0-255.", code)
		return false
	}

	return h.reallyPing(ctx, ppfmt, fmt.Sprintf("%s/%d", h.BaseURL, code), fmt.Sprintf("%s/%d", h.RedactedBaseURL, code))
}
