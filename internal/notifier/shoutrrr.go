package notifier

import (
	"context"
	"time"

	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/router"
	"github.com/containrrr/shoutrrr/pkg/types"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type Shoutrrr struct {
	// The router
	Router *router.ServiceRouter

	// The services
	ServiceNames []string
}

var _ Notifier = (*Shoutrrr)(nil)

const (
	// ShoutrrrDefaultTimeout is the default timeout for a UptimeKuma ping.
	ShoutrrrDefaultTimeout = 10 * time.Second
)

// NewShoutrrr creates a new shoutrrr notifier.
func NewShoutrrr(ppfmt pp.PP, rawURLs []string) (*Shoutrrr, bool) {
	r, err := shoutrrr.CreateSender(rawURLs...)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Could not create shoutrrr client: %v", err)
		return nil, false
	}

	r.Timeout = ShoutrrrDefaultTimeout

	serviceNames := make([]string, 0, len(rawURLs))
	for _, u := range rawURLs {
		s, _, _ := r.ExtractServiceName(u)
		serviceNames = append(serviceNames, s)
	}

	return &Shoutrrr{Router: r, ServiceNames: serviceNames}, true
}

func (s *Shoutrrr) Describe(callback func(service, params string)) {
	for _, n := range s.ServiceNames {
		callback(n, "(URL redacted)")
	}
}

func (s *Shoutrrr) Send(_ context.Context, ppfmt pp.PP, msg string) bool {
	errs := s.Router.Send(msg, &types.Params{})
	allOk := true
	for _, err := range errs {
		if err != nil {
			ppfmt.Errorf(pp.EmojiError, "Failed to send some shoutrrr message: %v", err)
			allOk = false
		}
	}
	if allOk {
		ppfmt.Infof(pp.EmojiNotification, "Sent shoutrrr message")
	}
	return allOk
}
