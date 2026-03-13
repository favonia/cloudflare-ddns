package provider

import (
	"net/url"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func newCustomURL(ppfmt pp.PP, providerName string, rawURL string, forcedTransportIPFamily *ipnet.Type) (Provider, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to parse the provider %s", providerName)
		return nil, false
	}

	if !u.IsAbs() || u.Opaque != "" || u.Host == "" {
		ppfmt.Noticef(pp.EmojiUserError, "The provider %s does not contain a valid URL", providerName)
		return nil, false
	}

	switch u.Scheme {
	case "http":
		ppfmt.Noticef(pp.EmojiUserWarning, "The provider %s uses HTTP; consider using HTTPS instead", providerName)

	case "https":
		// HTTPS is good!

	default:
		ppfmt.Noticef(pp.EmojiUserError, "The provider %s only supports HTTP and HTTPS", providerName)
		return nil, false
	}

	return protocol.HTTP{
		ProviderName: providerName,
		URL: map[ipnet.Type]string{
			ipnet.IP4: rawURL,
			ipnet.IP6: rawURL,
		},
		ForcedTransportIPFamily: forcedTransportIPFamily,
	}, true
}

// NewCustomURL creates a strict HTTP provider that matches the transport family
// to the managed IP family.
func NewCustomURL(ppfmt pp.PP, rawURL string) (Provider, bool) {
	return newCustomURL(ppfmt, "url:(redacted)", rawURL, nil)
}

// NewCustomURLVia4 creates a HTTP provider that always connects via IPv4.
func NewCustomURLVia4(ppfmt pp.PP, rawURL string) (Provider, bool) {
	forcedTransportIPFamily := ipnet.IP4
	return newCustomURL(ppfmt, "url.via4:(redacted)", rawURL, &forcedTransportIPFamily)
}

// NewCustomURLVia6 creates a HTTP provider that always connects via IPv6.
func NewCustomURLVia6(ppfmt pp.PP, rawURL string) (Provider, bool) {
	forcedTransportIPFamily := ipnet.IP6
	return newCustomURL(ppfmt, "url.via6:(redacted)", rawURL, &forcedTransportIPFamily)
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

// MustNewCustomURLVia4 creates a HTTP provider and panics if it fails.
func MustNewCustomURLVia4(rawURL string) Provider {
	var buf strings.Builder
	p, ok := NewCustomURLVia4(pp.NewDefault(&buf), rawURL)
	if !ok {
		panic(buf.String())
	}
	return p
}

// MustNewCustomURLVia6 creates a HTTP provider and panics if it fails.
func MustNewCustomURLVia6(rawURL string) Provider {
	var buf strings.Builder
	p, ok := NewCustomURLVia6(pp.NewDefault(&buf), rawURL)
	if !ok {
		panic(buf.String())
	}
	return p
}
