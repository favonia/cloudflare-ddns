package notifier

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/response"
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

// SendResponseAll calls [SendResponse] for each monitor in the group.
func SendResponseAll(ctx context.Context, ppfmt pp.PP, ns []Notifier, r response.Response) bool {
	ok := true
	for _, n := range ns {
		if !SendResponse(ctx, ppfmt, n, r) {
			ok = false
		}
	}
	return ok
}
