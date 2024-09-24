package provider

import "github.com/favonia/cloudflare-ddns/internal/provider/protocol"

// NewLocalWithInterface creates a protocol.LocalWithInterface provider.
func NewLocalWithInterface(iface string) Provider {
	return protocol.LocalWithInterface{
		ProviderName:  "local:" + iface,
		InterfaceName: iface,
	}
}
