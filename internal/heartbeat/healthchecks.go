package heartbeat

import (
	"context"
	"io"
	"net/http"
	"net/url"
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

var _ Heartbeat = Healthchecks{} //nolint:exhaustruct

const (
	// HealthchecksDefaultTimeout is the default timeout for a Healthchecks ping.
	HealthchecksDefaultTimeout = 10 * time.Second
)

type healthchecksPingSpec struct {
	endpoint                  string
	location                  string
	descriptionWithArticle    string
	descriptionWithoutArticle string
}

//nolint:gochecknoglobals // Structure values cannot be constants.
var (
	healthchecksPingDefault = healthchecksPingSpec{
		endpoint:                  "",
		location:                  "base URL",
		descriptionWithArticle:    "a ping",
		descriptionWithoutArticle: "ping",
	}
	healthchecksPingFailure = healthchecksPingSpec{
		endpoint:                  "/fail",
		location:                  "/fail",
		descriptionWithArticle:    "a failure ping",
		descriptionWithoutArticle: "failure ping",
	}
	healthchecksPingStart = healthchecksPingSpec{
		endpoint:                  "/start",
		location:                  "/start",
		descriptionWithArticle:    "a start ping",
		descriptionWithoutArticle: "start ping",
	}
	healthchecksPingExit = healthchecksPingSpec{
		endpoint:                  "/0",
		location:                  "/0",
		descriptionWithArticle:    "an exit ping",
		descriptionWithoutArticle: "exit ping",
	}
	healthchecksPingLog = healthchecksPingSpec{
		endpoint:                  "/log",
		location:                  "/log",
		descriptionWithArticle:    "a log ping",
		descriptionWithoutArticle: "log ping",
	}
)

// NewHealthchecks creates a new Healthchecks heartbeat service.
// See https://healthchecks.io/docs/http_api/ for more information.
func NewHealthchecks(ppfmt pp.PP, rawURL string) (Healthchecks, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to parse the Healthchecks URL (redacted)")
		return Healthchecks{}, false //nolint:exhaustruct
	}

	if !u.IsAbs() || u.Host == "" || u.Opaque != "" || u.RawQuery != "" {
		ppfmt.Noticef(pp.EmojiUserError, `The Healthchecks URL (redacted) is not a valid URL`)
		ppfmt.Noticef(pp.EmojiUserError, `Expected a URL like "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`)
		return Healthchecks{}, false //nolint:exhaustruct
	}

	switch u.Scheme {
	case "http":
		ppfmt.Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS")

	case "https":
		// HTTPS is good!

	default:
		ppfmt.Noticef(pp.EmojiUserError, `The Healthchecks URL (redacted) is not a valid URL`)
		ppfmt.Noticef(pp.EmojiUserError, `Expected a URL like "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`)
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
		return h.ping(ctx, ppfmt, healthchecksPingDefault, msg.Format())
	} else {
		return h.ping(ctx, ppfmt, healthchecksPingFailure, msg.Format())
	}
}

// Start pings the /start endpoint.
func (h Healthchecks) Start(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, healthchecksPingStart, message)
}

// Exit pings the /0 endpoint.
func (h Healthchecks) Exit(ctx context.Context, ppfmt pp.PP, message string) bool {
	return h.ping(ctx, ppfmt, healthchecksPingExit, message)
}

// Log formats and logs a [Message].
func (h Healthchecks) Log(ctx context.Context, ppfmt pp.PP, msg Message) bool {
	switch {
	case !msg.OK:
		return h.ping(ctx, ppfmt, healthchecksPingFailure, msg.Format())
	case !msg.IsEmpty():
		return h.ping(ctx, ppfmt, healthchecksPingLog, msg.Format())
	default:
		return true
	}
}

/*
ping sends a POST request to a Healthchecks server.

The caller is responsible for choosing a [healthchecksPingSpec], which defines
the variable user-facing wording for the request:

  - location is the endpoint label shown in error messages.
  - descriptionWithArticle is used in messages like "Successfully sent ... to Healthchecks".
  - descriptionWithoutArticle is used in messages like "The ... to Healthchecks returned an unexpected response".

This keeps the contract explicit while avoiding repeated string literals at
each call site.

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
func (h Healthchecks) ping(ctx context.Context, ppfmt pp.PP, spec healthchecksPingSpec, message string) bool {
	url := h.BaseURL.JoinPath(spec.endpoint)

	ctx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodPost, url.String(), strings.NewReader(message))
	if err != nil {
		ppfmt.Noticef(pp.EmojiImpossible,
			"Failed to create the request for %s to Healthchecks (%s): %v",
			spec.descriptionWithArticle, spec.location, err)
		return false
	}

	c := retryablehttp.NewClient()
	c.Logger = nil

	resp, err := c.Do(req)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to send %s to Healthchecks (%s): %v",
			spec.descriptionWithArticle, spec.location, err)
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReadLength))
	if err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to read the response from Healthchecks for %s (%s): %v",
			spec.descriptionWithArticle, spec.location, err)
		return false
	}

	bodyAsString := strings.TrimSpace(string(body))
	if resp.StatusCode != http.StatusOK || bodyAsString != "OK" {
		ppfmt.Noticef(pp.EmojiError,
			"The %s to Healthchecks returned an unexpected response (%s): got %d %s",
			spec.descriptionWithoutArticle, spec.location, resp.StatusCode, bodyAsString,
		)
		return false
	}

	ppfmt.Infof(pp.EmojiPing, "Successfully sent %s to Healthchecks", spec.descriptionWithArticle)
	return true
}
