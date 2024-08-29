package notifier

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/message"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// DescribeAll calls [Notifier.Describe] for each monitor in the group with the callback.
func DescribeAll(callback func(service, params string), ns []Notifier) {
	for _, n := range ns {
		n.Describe(callback)
	}
}

// SendAll calls [Notifier.Send] for each monitor in the group.
func SendAll(ctx context.Context, ppfmt pp.PP, ns []Notifier, message string) bool {
	ok := true
	for _, n := range ns {
		if !n.Send(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

// SendMessageAll calls [SendMessage] for each monitor in the group.
func SendMessageAll(ctx context.Context, ppfmt pp.PP, ns []Notifier, msg message.NotifierMessage) bool {
	ok := true
	for _, n := range ns {
		if !SendMessage(ctx, ppfmt, n, msg) {
			ok = false
		}
	}
	return ok
}
