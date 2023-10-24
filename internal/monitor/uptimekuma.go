package monitor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/google/go-querystring/query"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// UptimeKuma provides basic support of Uptime Kuma.
//
//   - ExitStatus, Start, and Log will be no-op.
//   - Success/Fail will be translated to status=up/down
//   - Messages will be sent along with Success/Fail,
//     but it seems Uptime Kuma will only display the first one.
//   - ping will always be empty
type UptimeKuma struct {
	// The endpoint
	BaseURL *url.URL

	// Timeout for each ping
	Timeout time.Duration
}

const (
	// UptimeKumaDefaultTimeout is the default timeout for a UptimeKuma ping.
	UptimeKumaDefaultTimeout = 10 * time.Second
)

// NewUptimeKuma creates a new UptimeKuma monitor.
func NewUptimeKuma(ppfmt pp.PP, rawURL string) (Monitor, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse the Uptime Kuma URL (redacted)")
		return nil, false
	}

	if !(u.IsAbs() && u.Opaque == "" && u.Host != "") {
		ppfmt.Errorf(pp.EmojiUserError, `The Uptime Kuma URL (redacted) does not look like a valid URL`)
		return nil, false
	}

	switch u.Scheme {
	case "http":
		ppfmt.Warningf(pp.EmojiUserWarning, "The Uptime Kuma URL (redacted) uses HTTP; please consider using HTTPS")

	case "https":
		// HTTPS is good!

	default:
		ppfmt.Errorf(pp.EmojiUserError, `The Uptime Kuma URL (redacted) does not look like a valid URL`)
		return nil, false
	}

	// By default, the URL provided by Uptime Kuma has this:
	//
	//     https://some.host.name/api/push/GFWB6vsHMg?status=up&msg=Ok&ping=
	//
	// The following will check the query part
	if u.RawQuery != "" {
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			ppfmt.Errorf(pp.EmojiUserError, `The Uptime Kuma URL (redacted) does not look like a valid URL`)
			return nil, false
		}

		for k, vs := range q {
			switch {
			case k == "status" && slices.Equal(vs, []string{"up"}): // status=up
			case k == "msg" && slices.Equal(vs, []string{"OK"}): // msg=OK
			case k == "ping" && slices.Equal(vs, []string{""}): // ping=

			default: // problematic case
				ppfmt.Warningf(pp.EmojiUserError,
					`The Uptime Kuma URL (redacted) contains an unexpected query %s=... and it will not be used`,
					k)
			}
		}

		// Clear all queries to obtain the base URL
		u.RawQuery = ""
	}

	h := &UptimeKuma{
		BaseURL: u,
		Timeout: UptimeKumaDefaultTimeout,
	}

	return h, true
}

// Describe calls the callback with the service name "Uptime Kuma".
func (h *UptimeKuma) Describe(callback func(service, params string)) {
	callback("Uptime Kuma", "(URL redacted)")
}

// UptimeKumaResponse is for parsing the response from Uptime Kuma.
type UptimeKumaResponse struct {
	Ok  bool   `json:"ok"`
	Msg string `json:"msg"`
}

// UptimeKumaRequest is for assembling the request to Uptime Kuma.
type UptimeKumaRequest struct {
	Status string `url:"status"`
	Msg    string `url:"msg"`
	Ping   string `url:"ping"`
}

func (h *UptimeKuma) ping(ctx context.Context, ppfmt pp.PP, param UptimeKumaRequest) bool {
	ctx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()

	url := *h.BaseURL
	v, _ := query.Values(param)
	url.RawQuery = v.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		ppfmt.Warningf(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to Uptime Kuma: %v", err)
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to send HTTP(S) request to Uptime Kuma: %v", err)
		return false
	}
	defer resp.Body.Close()

	var parsedResp UptimeKumaResponse
	if err = json.NewDecoder(resp.Body).Decode(&parsedResp); err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to parse the response from Uptime Kuma: %v", err)
		return false
	}
	if !parsedResp.Ok {
		ppfmt.Warningf(pp.EmojiError, "Failed to ping Uptime Kuma: %q", parsedResp.Msg)
		return false
	}

	ppfmt.Infof(pp.EmojiNotification, "Successfully pinged Uptime Kuma")
	return true
}

// Success pings the server with status=up. Messages are ignored and "OK" is used instead.
// The reason is that Uptime Kuma seems to show only the first success message
// and it could be misleading if an outdated message stays in the UI.
func (h *UptimeKuma) Success(ctx context.Context, ppfmt pp.PP, _message string) bool {
	return h.ping(ctx, ppfmt, UptimeKumaRequest{Status: "up", Msg: "OK", Ping: ""})
}

// Start does nothing.
func (h *UptimeKuma) Start(ctx context.Context, ppfmt pp.PP, _message string) bool {
	return true
}

// Failure pings the server with status=down.
func (h *UptimeKuma) Failure(ctx context.Context, ppfmt pp.PP, message string) bool {
	if len(message) == 0 {
		// If we do not send a non-empty message to Uptime Kuma, it seems to
		// either keep the previous message (even if it was for success) or
		// assume the message is "OK". Either is bad.
		//
		// We can send a non-empty message to overwrite it.
		message = "Failing"
	}
	return h.ping(ctx, ppfmt, UptimeKumaRequest{Status: "down", Msg: message, Ping: ""})
}

// Log does nothing.
func (h *UptimeKuma) Log(ctx context.Context, ppfmt pp.PP, _message string) bool {
	return true
}

// ExitStatus with non-zero triggers [Failure]. Otherwise, it does nothing.
func (h *UptimeKuma) ExitStatus(ctx context.Context, ppfmt pp.PP, code int, message string) bool {
	if code != 0 {
		return h.Failure(ctx, ppfmt, message)
	}
	return true
}
