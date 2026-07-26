package main

import "github.com/favonia/cloudflare-ddns/internal/notifier"

func startupFailureNotification() notifier.Notification {
	return notifier.NewNotificationf(notifier.KindStartupFailure,
		"Cloudflare DDNS was misconfigured and could not start. Please check the logs for details.")
}

func startupNotification() notifier.Notification {
	return notifier.NewNotificationf(notifier.KindStartup, "Cloudflare DDNS has started.")
}

// schedulingFailureNotification adds the given schedule to the message.
// The schedule is the original UPDATE_CRON string.
func schedulingFailureNotification(schedule string) notifier.Notification {
	return notifier.NewNotificationf(notifier.KindSchedulingFailure,
		"Cloudflare DDNS stopped because no updates are scheduled in the near future. "+
			"Consider changing the value of UPDATE_CRON (%s).",
		schedule)
}

func shutdownNotification() notifier.Notification {
	return notifier.NewNotificationf(notifier.KindShutdown, "Cloudflare DDNS has stopped.")
}
