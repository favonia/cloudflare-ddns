package config

import (
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ReadAndAppendShoutrrrURL reads the URLs separated by the newline.
func ReadAndAppendShoutrrrURL(ppfmt pp.PP, key string, field *notifier.Notifier) bool {
	vals := GetenvAsList(key, "\n")
	if len(vals) == 0 {
		return true
	}

	ppfmt.Hintf(pp.HintExperimentalShoutrrr,
		"You are using the experimental shoutrrr support added in version 1.12.0")

	s, ok := notifier.NewShoutrrr(ppfmt, vals)
	if !ok {
		return false
	}

	// Append the new monitor to the existing list
	*field = notifier.NewComposed(*field, s)
	return true
}
