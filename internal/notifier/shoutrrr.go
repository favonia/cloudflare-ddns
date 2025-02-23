package notifier

import (
	"context"
	"time"

	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/router"
	"github.com/containrrr/shoutrrr/pkg/types"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Shoutrrr wraps a handler of a shoutrrr router.
type Shoutrrr struct {
	// The router
	Router *router.ServiceRouter

	// The services
	ServiceDescriptions []string
}

var _ Notifier = Shoutrrr{} //nolint:exhaustruct

const (
	// ShoutrrrDefaultTimeout is the default timeout for a UptimeKuma ping.
	ShoutrrrDefaultTimeout = 10 * time.Second
)

// DescribeShoutrrrService gives a human-readable description for a service.
func DescribeShoutrrrService(ppfmt pp.PP, proto string) string {
	name, known := map[string]string{
		"bark":       "Bark",
		"discord":    "Discord",
		"smtp":       "Email",
		"gotify":     "Gotify",
		"googlechat": "Google Chat",
		"ifttt":      "IFTTT",
		"join":       "Join",
		"mattermost": "Mattermost",
		"matrix":     "Matrix",
		"ntfy":       "Ntfy",
		"opsgenie":   "OpsGenie",
		"pushbullet": "Pushbullet",
		"pushover":   "Pushover",
		"rocketchat": "Rocketchat",
		"slack":      "Slack",
		"teams":      "Teams",
		"telegram":   "Telegram",
		"zulip":      "Zulip Chat",
		"generic":    "Generic",
	}[proto]

	if known {
		return name
	} else {
		ppfmt.Noticef(pp.EmojiImpossible,
			"Unknown shoutrrr service name %q; please report it at %s",
			name, pp.IssueReportingURL)
		return cases.Title(language.English).String(proto)
	}
}

// NewShoutrrr creates a new shoutrrr notifier.
func NewShoutrrr(ppfmt pp.PP, rawURLs []string) (Shoutrrr, bool) {
	r, err := shoutrrr.CreateSender(rawURLs...)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Could not create shoutrrr client: %v", err)
		return Shoutrrr{}, false //nolint:exhaustruct
	}

	r.Timeout = ShoutrrrDefaultTimeout

	serviceDescriptions := make([]string, 0, len(rawURLs))
	for _, u := range rawURLs {
		s, _, _ := r.ExtractServiceName(u)
		serviceDescriptions = append(serviceDescriptions, DescribeShoutrrrService(ppfmt, s))
	}

	return Shoutrrr{Router: r, ServiceDescriptions: serviceDescriptions}, true
}

// Describe calls callback on each registered notification service.
func (s Shoutrrr) Describe(yield func(string, string) bool) {
	for _, n := range s.ServiceDescriptions {
		if !yield(n, "(URL redacted)") {
			return
		}
	}
}

// Send sents the message msg.
func (s Shoutrrr) Send(_ context.Context, ppfmt pp.PP, msg Message) bool {
	if msg.IsEmpty() {
		return true
	}

	formattedMsg := msg.Format()
	errs := s.Router.Send(formattedMsg, &types.Params{})
	allOK := true
	for _, err := range errs {
		if err != nil {
			ppfmt.Noticef(pp.EmojiError,
				"Failed to notify shoutrrr service(s): %v (attempted to send: %s)",
				err,
				formattedMsg,
			)
			allOK = false
		}
	}
	if allOK {
		ppfmt.Infof(pp.EmojiNotify,
			"Notified %s via shoutrrr: %s",
			pp.EnglishJoin(s.ServiceDescriptions),
			formattedMsg,
		)
	}
	return allOK
}
