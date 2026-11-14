package main

import "github.com/favonia/cloudflare-ddns/internal/notifier"

func startupFailureNotification() notifier.Notification {
	return notifier.NewNotificationf(notifier.KindStartupFailure,
		"Cloudflare DDNS was misconfigured and could not start. Please check the logs for details.")
}
