package notifier

import (
	"context"
	"strconv"
	"time"

	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/router"
	"github.com/containrrr/shoutrrr/pkg/types"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Shoutrrr wraps a handler of a shoutrrr router.
type Shoutrrr struct {
	// The router
	Router *router.ServiceRouter

	// The services
	ServiceNames []string
}

var _ Notifier = Shoutrrr{} //nolint:exhaustruct

const (
	// ShoutrrrDefaultTimeout is the default timeout for a UptimeKuma ping.
	ShoutrrrDefaultTimeout = 10 * time.Second
)

// NewShoutrrr creates a new shoutrrr notifier.
func NewShoutrrr(ppfmt pp.PP, rawURLs []string) (Shoutrrr, bool) {
	r, err := shoutrrr.CreateSender(rawURLs...)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Could not create shoutrrr client: %v", err)
		return Shoutrrr{}, false //nolint:exhaustruct
	}

	r.Timeout = ShoutrrrDefaultTimeout

	serviceNames := make([]string, 0, len(rawURLs))
	for _, u := range rawURLs {
		s, _, _ := r.ExtractServiceName(u)
		serviceNames = append(serviceNames, s)
	}

	return Shoutrrr{Router: r, ServiceNames: serviceNames}, true
}

// Describe calls callback on each registered notification service.
func (s Shoutrrr) Describe(callback func(service, params string)) {
	for _, n := range s.ServiceNames {
		callback(n, "(URL redacted)")
	}
}

// Send sents the message msg.
func (s Shoutrrr) Send(_ context.Context, ppfmt pp.PP, msg string) bool {
	errs := s.Router.Send(msg, &types.Params{})
	allOK := true
	for _, err := range errs {
		if err != nil {
			ppfmt.Noticef(pp.EmojiError, "Failed to notify shoutrrr service(s): %v", err)
			allOK = false
		}
	}
	if allOK {
		ppfmt.Infof(pp.EmojiNotify, "Notified %s via shoutrrr", pp.EnglishJoinMap(strconv.Quote, s.ServiceNames))
	}
	return allOK
}
