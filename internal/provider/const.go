package provider

import (
	"net/netip"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewDebugConst creates a [protocol.Const] provider.
func NewDebugConst(ppfmt pp.PP, raw string) (Provider, bool) {
	ip, err := netip.ParseAddr(raw)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, `Failed to parse the IP address %q for "debug.const:"`, raw)
		return nil, false
	}
	if ip.Zone() != "" {
		ppfmt.Noticef(
			pp.EmojiUserError,
			`Failed to parse the IP address %q for "debug.const:": zoned IP addresses are not allowed`,
			raw,
		)
		return nil, false
	}

	return protocol.Const{
		ProviderName: "debug.const:" + ip.String(),
		IP:           ip,
	}, true
}

// MustNewDebugConst creates a [protocol.Const] provider and panics if it fails.
func MustNewDebugConst(raw string) Provider {
	var buf strings.Builder
	p, ok := NewDebugConst(pp.NewDefault(&buf), raw)
	if !ok {
		panic(buf.String())
	}
	return p
}
