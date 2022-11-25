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

// Healthchecks represents a Healthchecks access point.
// See https://healthchecks.io/docs/http_api/ for more information.
type Healthchecks struct {
	// The success endpoint that can be used to derive all other endpoints.
	BaseURL *url.URL

	// Timeout for each ping.
	Timeout time.Duration

	// MaxRetries before giving up a ping.
	MaxRetries int
}

const (
	// HealthchecksDefaultTimeout is the default timeout for a Healthchecks ping.
	HealthchecksDefaultTimeout = 10 * time.Second

	// HealthchecksDefaultMaxRetries is the maximum number of retries to ping Healthchecks.
	HealthchecksDefaultMaxRetries int = 5
)

// HealthchecksOption is an option for [NewHealthchecks].
type HealthchecksOption func(*Healthchecks)

// SetHealthchecksMaxRetries sets the MaxRetries of a Healthchecks.
func SetHealthchecksMaxRetries(maxRetries int) HealthchecksOption {
	if maxRetries <= 0 {
		panic("maxRetries <= 0")
	}
	return func(h *Healthchecks) {
		h.MaxRetries = maxRetries
	}
}

// NewHealthchecks creates a new Healthchecks monitor.
// See https://healthchecks.io/docs/http_api/ for more information.
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
		ppfmt.Errorf(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL.`)
		ppfmt.Errorf(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc".`)
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

// Describe calles the callback with the service name "Healthchecks".
func (h *Healthchecks) Describe(callback func(service, params string)) {
	callback("Healthchecks", "(URL redacted)")
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

// Success pings the root endpoint.
func (h *Healthchecks) Success(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, "", message)
}

// Start pings the /start endpoint.
func (h *Healthchecks) Start(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, "/start", message)
}

// Failure pings the /fail endpoint.
func (h *Healthchecks) Failure(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, "/fail", message)
}

// Log pings the /log endpoint.
func (h *Healthchecks) Log(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, "/log", message)
}

// ExitStatus pings the /number endpoint where number is the exit status.
func (h *Healthchecks) ExitStatus(ctx context.Context, ppfmt pp.PP, code int, message string) bool {
	if code < 0 || code > 255 {
		ppfmt.Errorf(pp.EmojiImpossible, "Exit code (%d) not within the range 0-255", code)
		return false
	}

	return h.ping(ctx, ppfmt, fmt.Sprintf("/%d", code), message)
}
