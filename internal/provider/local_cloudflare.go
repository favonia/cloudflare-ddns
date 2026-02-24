package provider

import "github.com/favonia/cloudflare-ddns/internal/provider/protocol"

// NewLocal creates a specialized Local provider that uses Cloudflare as the remote server.
// (No actual UDP packets will be sent to Cloudflare.)
func NewLocal() Provider {
	return protocol.LocalAuto{
		ProviderName:  "local",
		RemoteUDPAddr: "api.cloudflare.com:443",
	}
}
