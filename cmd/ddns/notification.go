package main

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func startupFailureNotification() notifier.Notification {
	return notifier.NewNotificationf(notifier.KindStartupFailure,
		"Cloudflare DDNS was misconfigured and could not start. Please check the logs for details.")
}

// sendNotification skips empty one-shot notifications. Delivery does not change
// the work result; scheduled submission bookkeeping belongs to its reporter.
func sendNotification(ctx context.Context, ppfmt pp.PP, nt notifier.Notifier,
	notification notifier.Notification,
) {
	if !notification.IsEmpty() {
		nt.Send(ctx, ppfmt, notification)
	}
}
