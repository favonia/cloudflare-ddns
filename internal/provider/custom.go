package provider

import (
	"net/url"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewCustomURL creates a HTTP provider.
func NewCustomURL(ppfmt pp.PP, rawURL string) (Provider, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, `Failed to parse the provider url:(redacted)`)
		return nil, false
	}

	if !u.IsAbs() || u.Opaque != "" || u.Host == "" {
		ppfmt.Noticef(pp.EmojiUserError, `The provider url:(redacted) does not contain a valid URL`)
		return nil, false
	}

	switch u.Scheme {
	case "http":
		ppfmt.Noticef(pp.EmojiUserWarning, "The provider url:(redacted) uses HTTP; consider using HTTPS instead")

	case "https":
		// HTTPS is good!

	default:
		ppfmt.Noticef(pp.EmojiUserError, `The provider url:(redacted) only supports HTTP and HTTPS`)
		return nil, false
	}

	return protocol.HTTP{
		ProviderName: "url:(redacted)",
		URL: map[ipnet.Type]string{
			ipnet.IP4: rawURL,
			ipnet.IP6: rawURL,
		},
	}, true
}

// MustNewCustomURL creates a HTTP provider and panics if it fails.
func MustNewCustomURL(rawURL string) Provider {
	var buf strings.Builder
	p, ok := NewCustomURL(pp.NewDefault(&buf), rawURL)
	if !ok {
		panic(buf.String())
	}
	return p
}
