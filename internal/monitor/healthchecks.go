package monitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type HealthChecks struct {
	BaseURL    *url.URL
	Timeout    time.Duration
	MaxRetries int
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
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse the Healthchecks.io URL (redacted).")
		return nil, false
	}

	if !(url.IsAbs() && url.Opaque == "" && url.Host != "") { //nolint:lll
		ppfmt.Errorf(pp.EmojiUserError, `The Healthchecks.io URL (redacted) does not look like a valid URL.`)
		ppfmt.Errorf(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc".`)
		return nil, false
	}

	h := &HealthChecks{
		BaseURL:    url,
		Timeout:    HealthChecksDefaultTimeout,
		MaxRetries: HealthChecksDefaultMaxRetries,
	}

	for _, o := range os {
		o(h)
	}

	return h, true
}

func (h *HealthChecks) DescribeService() string {
	return "Healthchecks.io"
}

//nolint:funlen
func (h *HealthChecks) ping(ctx context.Context, ppfmt pp.PP, endpoint string) bool {
	url := h.BaseURL.JoinPath(endpoint)

	endpointDescription := "default (root)"
	if endpoint != "" {
		endpointDescription = strconv.Quote(endpoint)
	}

	for retries := 0; retries < h.MaxRetries; retries++ {
		if retries > 0 {
			time.Sleep(time.Second << (retries - 1))
		}

		ctx, cancel := context.WithTimeout(ctx, h.Timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
		if err != nil {
			ppfmt.Warningf(pp.EmojiImpossible,
				"Failed to prepare HTTP(S) request to the %s endpoint of Healthchecks.io: %v",
				endpointDescription, err)
			return false
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			ppfmt.Warningf(pp.EmojiError,
				"Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v",
				endpointDescription, err)
			ppfmt.Infof(pp.EmojiRepeatOnce, "Trying again . . .")
			continue
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			ppfmt.Warningf(pp.EmojiError,
				"Failed to read HTTP(S) response from the %s endpoint of Healthchecks.io: %v",
				endpointDescription, err)
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
			ppfmt.Warningf(pp.EmojiError,
				"Failed to ping the %s endpoint of Healthchecks.io; got response code: %d %s",
				endpointDescription, resp.StatusCode, bodyAsString,
			)
			return false
		}

		ppfmt.Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks.io.", endpointDescription)
		return true
	}

	ppfmt.Warningf(
		pp.EmojiError,
		"Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io in %d time(s).",
		endpointDescription, h.MaxRetries)
	return false
}

func (h *HealthChecks) Success(ctx context.Context, ppfmt pp.PP) bool {
	return h.ping(ctx, ppfmt, "")
}

func (h *HealthChecks) Start(ctx context.Context, ppfmt pp.PP) bool {
	return h.ping(ctx, ppfmt, "/start")
}

func (h *HealthChecks) Failure(ctx context.Context, ppfmt pp.PP) bool {
	return h.ping(ctx, ppfmt, "/fail")
}

func (h *HealthChecks) ExitStatus(ctx context.Context, ppfmt pp.PP, code int) bool {
	if code < 0 || code > 255 {
		ppfmt.Errorf(pp.EmojiImpossible, "Exit code (%i) not within the range 0-255.", code)
		return false
	}

	return h.ping(ctx, ppfmt, fmt.Sprintf("/%d", code))
}
