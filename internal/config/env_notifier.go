package config

import (
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ReadAndAppendShoutrrrURL reads the URLs separated by the newline.
func ReadAndAppendShoutrrrURL(ppfmt pp.PP, key string, field *[]notifier.Notifier) bool {
	vals := Getenvs(key)

	if len(vals) == 0 {
		return true
	}

	s, ok := notifier.NewShoutrrr(ppfmt, vals)
	if !ok {
		return false
	}

	// Append the new monitor to the existing list
	*field = append(*field, s)
	return true
}
