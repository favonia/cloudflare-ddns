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

type Healthchecks struct {
	BaseURL    *url.URL
	Timeout    time.Duration
	MaxRetries int
}

const (
	HealthchecksDefaultTimeout    = 10 * time.Second
	HealthchecksDefaultMaxRetries = 5
)

type HealthchecksOption func(*Healthchecks)

func SetHealthchecksMaxRetries(maxRetries int) HealthchecksOption {
	if maxRetries <= 0 {
		panic("maxRetries <= 0")
	}
	return func(h *Healthchecks) {
		h.MaxRetries = maxRetries
	}
}

func NewHealthchecks(ppfmt pp.PP, rawURL string, os ...HealthchecksOption) (Monitor, bool) {
	url, err := url.Parse(rawURL)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse the Healthchecks URL (redacted)")
		return nil, false
	}

	if !(url.IsAbs() && url.Opaque == "" && url.Host != "") {
		ppfmt.Errorf(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL.`)
		ppfmt.Errorf(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc".`)
		return nil, false
	}

	switch url.Scheme {
	case "http":
		ppfmt.Warningf(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS")

	case "https":
		// HTTPS is good!

	default:
		ppfmt.Errorf(pp.EmojiUserError, "The Healthchecks URL (redacted) does not use HTTP(S) and is not supported")
		return nil, false
	}

	h := &Healthchecks{
		BaseURL:    url,
		Timeout:    HealthchecksDefaultTimeout,
		MaxRetries: HealthchecksDefaultMaxRetries,
	}

	for _, o := range os {
		o(h)
	}

	return h, true
}

func (h *Healthchecks) DescribeService() string {
	return "Healthchecks"
}

//nolint:funlen
func (h *Healthchecks) ping(ctx context.Context, ppfmt pp.PP, endpoint string, message string) bool {
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

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), strings.NewReader(message))
		if err != nil {
			ppfmt.Warningf(pp.EmojiImpossible,
				"Failed to prepare HTTP(S) request to the %s endpoint of Healthchecks: %v",
				endpointDescription, err)
			return false
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			ppfmt.Warningf(pp.EmojiError,
				"Failed to send HTTP(S) request to the %s endpoint of Healthchecks: %v",
				endpointDescription, err)
			ppfmt.Infof(pp.EmojiRepeatOnce, "Trying again . . .")
			continue
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			ppfmt.Warningf(pp.EmojiError,
				"Failed to read HTTP(S) response from the %s endpoint of Healthchecks: %v",
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
				"Failed to ping the %s endpoint of Healthchecks; got response code: %d %s",
				endpointDescription, resp.StatusCode, bodyAsString,
			)
			return false
		}

		ppfmt.Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks", endpointDescription)
		return true
	}

	ppfmt.Warningf(
		pp.EmojiError,
		"Failed to send HTTP(S) request to the %s endpoint of Healthchecks in %d time(s)",
		endpointDescription, h.MaxRetries)
	return false
}

func (h *Healthchecks) Success(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, "", message)
}

func (h *Healthchecks) Start(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, "/start", message)
}

func (h *Healthchecks) Failure(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, "/fail", message)
}

func (h *Healthchecks) Log(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, "/log", message)
}

func (h *Healthchecks) ExitStatus(ctx context.Context, ppfmt pp.PP, code int, message string) bool {
	if code < 0 || code > 255 {
		ppfmt.Errorf(pp.EmojiImpossible, "Exit code (%d) not within the range 0-255", code)
		return false
	}

	return h.ping(ctx, ppfmt, fmt.Sprintf("/%d", code), message)
}
