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

	"github.com/hashicorp/go-retryablehttp"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Healthchecks represents a Healthchecks access point.
// See https://healthchecks.io/docs/http_api/ for more information.
type Healthchecks struct {
	// The success endpoint that can be used to derive all other endpoints.
	BaseURL *url.URL

	// Timeout for each ping.
	Timeout time.Duration
}

const (
	// HealthchecksDefaultTimeout is the default timeout for a Healthchecks ping.
	HealthchecksDefaultTimeout = 10 * time.Second
)

// NewHealthchecks creates a new Healthchecks monitor.
// See https://healthchecks.io/docs/http_api/ for more information.
func NewHealthchecks(ppfmt pp.PP, rawURL string) (Monitor, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse the Healthchecks URL (redacted)")
		return nil, false
	}

	if !(u.IsAbs() && u.Opaque == "" && u.Host != "" && u.RawQuery == "") {
		ppfmt.Errorf(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL`)
		ppfmt.Errorf(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`)
		return nil, false
	}

	switch u.Scheme {
	case "http":
		ppfmt.Warningf(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS")

	case "https":
		// HTTPS is good!

	default:
		ppfmt.Errorf(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL`)
		ppfmt.Errorf(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`)
		return nil, false
	}

	h := &Healthchecks{
		BaseURL: u,
		Timeout: HealthchecksDefaultTimeout,
	}

	return h, true
}

// Describe calls the callback with the service name "Healthchecks".
func (h *Healthchecks) Describe(callback func(service, params string)) {
	callback("Healthchecks", "(URL redacted)")
}

/*
ping sends a POST request to a Healthchecks server.

Code and body for UUID API:

  - 200
    OK
  - 200
    OK (not found)
  - 200
    OK (rate limited)
  - 400
    invalid url format

Code and body for the slug API:

  - 200
    OK
  - 400
    invalid url format
  - 404
    not found
  - 409
    ambiguous slug
  - 429
    rate limit exceeded

To support both APIs, we check both (1) whether the status code is 200 and (2) whether the body is "OK".
*/
func (h *Healthchecks) ping(ctx context.Context, ppfmt pp.PP, endpoint string, message string) bool {
	url := h.BaseURL.JoinPath(endpoint)

	endpointDescription := "default (root)"
	if endpoint != "" {
		endpointDescription = strconv.Quote(endpoint)
	}

	ctx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodPost, url.String(), strings.NewReader(message))
	if err != nil {
		ppfmt.Warningf(pp.EmojiImpossible,
			"Failed to prepare HTTP(S) request to the %s endpoint of Healthchecks: %v",
			endpointDescription, err)
		return false
	}

	c := retryablehttp.NewClient()
	c.Logger = nil

	resp, err := c.Do(req)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError,
			"Failed to send HTTP(S) request to the %s endpoint of Healthchecks: %v",
			endpointDescription, err)
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReadLength))
	if err != nil {
		ppfmt.Warningf(pp.EmojiError,
			"Failed to read HTTP(S) response from the %s endpoint of Healthchecks: %v",
			endpointDescription, err)
		return false
	}

	bodyAsString := strings.TrimSpace(string(body))
	if resp.StatusCode != http.StatusOK || bodyAsString != "OK" {
		ppfmt.Warningf(pp.EmojiError,
			"Failed to ping the %s endpoint of Healthchecks; got response code: %d %s",
			endpointDescription, resp.StatusCode, bodyAsString,
		)
		return false
	}

	ppfmt.Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks", endpointDescription)
	return true
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
