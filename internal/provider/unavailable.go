package provider

import "github.com/favonia/cloudflare-ddns/internal/provider/protocol"

// NewDebugUnavailable creates a synthetic [protocol.Unavailable] provider
// that always reports detection as unavailable.
func NewDebugUnavailable() Provider {
	return protocol.Unavailable{ProviderName: "debug.unavailable"}
}
