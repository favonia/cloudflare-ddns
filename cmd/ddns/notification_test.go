package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/notifier"
)

func TestLifecycleNotifications(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		build       func() notifier.Notification
		wantKind    notifier.Kind
		wantPayload string
	}{
		"startup failure": {
			startupFailureNotification,
			notifier.KindStartupFailure,
			"Cloudflare DDNS was misconfigured and could not start. Please check the logs for details.",
		},
		"startup": {
			startupNotification,
			notifier.KindStartup,
			"Cloudflare DDNS has started.",
		},
		"scheduling failure": {
			func() notifier.Notification { return schedulingFailureNotification("@every 5m") },
			notifier.KindSchedulingFailure,
			"Cloudflare DDNS stopped because no updates are scheduled in the near future. " +
				"Consider changing the value of UPDATE_CRON (@every 5m).",
		},
		"shutdown": {
			shutdownNotification,
			notifier.KindShutdown,
			"Cloudflare DDNS has stopped.",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := tc.build()
			require.Equal(t, tc.wantKind, got.Kind)
			require.Equal(t, tc.wantPayload, got.Format())
		})
	}
}
