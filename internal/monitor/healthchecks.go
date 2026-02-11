package monitor

import (
	"context"
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

var _ Monitor = Healthchecks{} //nolint:exhaustruct

const (
	// HealthchecksDefaultTimeout is the default timeout for a Healthchecks ping.
	HealthchecksDefaultTimeout = 10 * time.Second
)

// NewHealthchecks creates a new Healthchecks monitor.
// See https://healthchecks.io/docs/http_api/ for more information.
func NewHealthchecks(ppfmt pp.PP, rawURL string) (Healthchecks, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to parse the Healthchecks URL (redacted)")
		return Healthchecks{}, false //nolint:exhaustruct
	}

	if !u.IsAbs() || u.Host == "" || u.Opaque != "" || u.RawQuery != "" {
		ppfmt.Noticef(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL`)
		ppfmt.Noticef(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`)
		return Healthchecks{}, false //nolint:exhaustruct
	}

	switch u.Scheme {
	case "http":
		ppfmt.Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS")

	case "https":
		// HTTPS is good!

	default:
		ppfmt.Noticef(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL`)
		ppfmt.Noticef(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`)
		return Healthchecks{}, false //nolint:exhaustruct
	}

	h := Healthchecks{
		BaseURL: u,
		Timeout: HealthchecksDefaultTimeout,
	}

	return h, true
}

// Describe calls the callback with the service name "Healthchecks".
func (h Healthchecks) Describe(yield func(service, params string) bool) {
	yield("Healthchecks", "(URL redacted)")
}

// Ping formats and pings with a [Message].
func (h Healthchecks) Ping(ctx context.Context, ppfmt pp.PP, msg Message) bool {
	if msg.OK {
		return h.ping(ctx, ppfmt, "", msg.Format())
	} else {
		return h.ping(ctx, ppfmt, "/fail", msg.Format())
	}
}

// Start pings the /start endpoint.
func (h Healthchecks) Start(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, "/start", message)
}

// Exit pings the /0 endpoint.
func (h Healthchecks) Exit(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, "/0", message)
}

// Log formats and logs a [Message].
func (h Healthchecks) Log(ctx context.Context, ppfmt pp.PP, msg Message) bool {
	switch {
	case !msg.OK:
		return h.ping(ctx, ppfmt, "/fail", msg.Format())
	case !msg.IsEmpty():
		return h.ping(ctx, ppfmt, "/log", msg.Format())
	default:
		return true
	}
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
func (h Healthchecks) ping(ctx context.Context, ppfmt pp.PP, endpoint string, message string) bool {
	url := h.BaseURL.JoinPath(endpoint)

	endpointDescription := "default (root)"
	if endpoint != "" {
		endpointDescription = strconv.Quote(endpoint)
	}

	ctx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodPost, url.String(), strings.NewReader(message))
	if err != nil {
		ppfmt.Noticef(pp.EmojiImpossible,
			"Failed to prepare HTTP(S) request to the %s endpoint of Healthchecks: %v",
			endpointDescription, err)
		return false
	}

	c := retryablehttp.NewClient()
	c.Logger = nil

	resp, err := c.Do(req)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to send HTTP(S) request to the %s endpoint of Healthchecks: %v",
			endpointDescription, err)
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReadLength))
	if err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to read HTTP(S) response from the %s endpoint of Healthchecks: %v",
			endpointDescription, err)
		return false
	}

	bodyAsString := strings.TrimSpace(string(body))
	if resp.StatusCode != http.StatusOK || bodyAsString != "OK" {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to ping the %s endpoint of Healthchecks; got response code: %d %s",
			endpointDescription, resp.StatusCode, bodyAsString,
		)
		return false
	}

	ppfmt.Infof(pp.EmojiPing, "Pinged the %s endpoint of Healthchecks", endpointDescription)
	return true
}
