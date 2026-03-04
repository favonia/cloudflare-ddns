package config

import (
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// SetupReporters reads and constructs the configured heartbeat and notifier
// services used by the updater process.
//
// This is a bootstrap path parallel to [RawConfig.ReadEnv] and
// [RawConfig.BuildConfig], not part of them. Its job is limited to the
// reporter-specific environment variables HEALTHCHECKS, UPTIMEKUMA, and
// SHOUTRRR.
func SetupReporters(ppfmt pp.PP) (heartbeat.Heartbeat, notifier.Notifier, bool) {
	emptyHeartbeat := heartbeat.NewComposed()
	emptyNotifier := notifier.NewComposed()
	hb := emptyHeartbeat
	nt := emptyNotifier

	if healthchecksURL := Getenv("HEALTHCHECKS"); healthchecksURL != "" {
		h, ok := heartbeat.NewHealthchecks(ppfmt, healthchecksURL)
		if !ok {
			return emptyHeartbeat, emptyNotifier, false
		}
		hb = heartbeat.NewComposed(hb, h)
	}

	if uptimeKumaURL := Getenv("UPTIMEKUMA"); uptimeKumaURL != "" {
		h, ok := heartbeat.NewUptimeKuma(ppfmt, uptimeKumaURL)
		if !ok {
			return emptyHeartbeat, emptyNotifier, false
		}
		hb = heartbeat.NewComposed(hb, h)
	}

	shoutrrrURLs := GetenvAsList("SHOUTRRR", "\n")
	if len(shoutrrrURLs) > 0 {
		ppfmt.InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint,
			"You are using the experimental shoutrrr support added in version 1.12.0")

		s, ok := notifier.NewShoutrrr(ppfmt, shoutrrrURLs)
		if !ok {
			return emptyHeartbeat, emptyNotifier, false
		}
		nt = notifier.NewComposed(nt, s)
	}

	return hb, nt, true
}
