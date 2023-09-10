package provider

import (
	"net/url"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCustom creates a HTTP provider.
func NewCustom(ppfmt pp.PP, rawURL string) (Provider, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse the custom provider (redacted)")
		return nil, false
	}

	if !u.IsAbs() || u.Opaque != "" || u.Host == "" {
		ppfmt.Errorf(pp.EmojiUserError, `The custom provider (redacted) does not look like a valid URL`)
		return nil, false
	}

	switch u.Scheme {
	case "http":
		ppfmt.Warningf(pp.EmojiUserWarning, "The custom provider (redacted) uses HTTP; consider using HTTPS instead")

	case "https":
		// HTTPS is good!

	default:
		ppfmt.Errorf(pp.EmojiUserError, `The custom provider (redacted) must use HTTP or HTTPS`)
		return nil, false
	}

	return &protocol.HTTP{
		ProviderName:     "custom",
		Is1111UsedForIP4: false,
		URL: map[ipnet.Type]protocol.Switch{
			ipnet.IP4: protocol.Constant(rawURL),
			ipnet.IP6: protocol.Constant(rawURL),
		},
	}, true
}

// MustNewCustom creates a HTTP provider and panics if it fails.
func MustNewCustom(rawURL string) Provider {
	var buf strings.Builder
	p, ok := NewCustom(pp.New(&buf), rawURL)
	if !ok {
		panic(buf.String())
	}
	return p
}
