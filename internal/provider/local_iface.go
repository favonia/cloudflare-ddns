package provider

import (
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewLocalWithInterface creates a protocol.LocalWithInterface provider.
func NewLocalWithInterface(ppfmt pp.PP, envKey string, iface string) (Provider, bool) {
	if strings.TrimSpace(iface) == "" {
		ppfmt.Noticef(
			pp.EmojiUserError,
			`%s=local.iface: must be followed by a network interface name`,
			envKey,
		)
		return nil, false
	}

	return protocol.LocalWithInterface{
		ProviderName:  "local.iface:" + iface,
		InterfaceName: iface,
	}, true
}

// MustNewLocalWithInterface creates a LocalWithInterface provider and panics if it fails.
func MustNewLocalWithInterface(iface string) Provider {
	var buf strings.Builder
	p, ok := NewLocalWithInterface(pp.NewDefault(&buf), "IP_PROVIDER", iface)
	if !ok {
		panic(buf.String())
	}
	return p
}
