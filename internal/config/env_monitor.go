package config

import (
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ReadAndAppendHealthchecksURL reads the base URL of a Healthchecks endpoint.
func ReadAndAppendHealthchecksURL(ppfmt pp.PP, key string, field *[]monitor.Monitor) bool {
	val := Getenv(key)

	if val == "" {
		return true
	}

	h, ok := monitor.NewHealthchecks(ppfmt, val)
	if !ok {
		return false
	}

	// Append the new monitor to the existing list
	*field = append(*field, h)
	return true
}
