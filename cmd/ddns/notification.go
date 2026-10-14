package main

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/updater"
)

func startupFailureNotification() notifier.Notification {
	return notifier.NewNotificationf(notifier.KindStartupFailure,
		"Cloudflare DDNS was misconfigured and could not start. Please check the logs for details.")
}

// submitNotification commits submitted coverage immediately before delivery.
// Empty envelopes do not reach the transport; delivery does not affect coverage.
func submitNotification(ctx context.Context, ppfmt pp.PP, nt notifier.Notifier,
	state *updater.ReportingState, envelope updater.Envelope,
) {
	if envelope.IsEmpty() {
		return
	}
	state.Submitted(envelope)
	nt.Send(ctx, ppfmt, envelope.Notification())
}
