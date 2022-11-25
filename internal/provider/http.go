package provider

import (
	"context"
	"io"
	"net/http"
	"net/netip"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type httpConn struct {
	url         string
	method      string
	contentType string
	accept      string
	reader      io.Reader
	extract     func(pp.PP, []byte) (netip.Addr, bool)
}

func (d *httpConn) getIP(ctx context.Context, ppfmt pp.PP) (netip.Addr, bool) {
	var invalidIP netip.Addr

	req, err := retryablehttp.NewRequestWithContext(ctx, d.method, d.url, d.reader)
	if err != nil {
		ppfmt.Warningf(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v", d.url, err)
		return invalidIP, false
	}

	if d.contentType != "" {
		req.Header.Set("Content-Type", d.contentType)
	}

	if d.accept != "" {
		req.Header.Set("Accept", d.accept)
	}

	c := retryablehttp.NewClient()
	c.Logger = nil

	resp, err := c.Do(req)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", d.url, err)
		return invalidIP, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to read HTTP(S) response from %q: %v", d.url, err)
		return invalidIP, false
	}

	return d.extract(ppfmt, body)
}

func getIPFromHTTP(ctx context.Context, ppfmt pp.PP, url string) (netip.Addr, bool) {
	c := httpConn{
		url:         url,
		method:      http.MethodGet,
		contentType: "",
		accept:      "",
		reader:      nil,
		extract: func(_ pp.PP, body []byte) (netip.Addr, bool) {
			ipString := string(body)
			ip, err := netip.ParseAddr(ipString)
			if err != nil {
				ppfmt.Errorf(pp.EmojiImpossible, `Failed to parse the IP address in the response of %q: %s`, url, ipString)
				return netip.Addr{}, false
			}
			return ip, true
		},
	}

	return c.getIP(ctx, ppfmt)
}

type HTTP struct {
	ProviderName string
	URL          map[ipnet.Type]string
}

func (p *HTTP) Name() string {
	return p.ProviderName
}

func (p *HTTP) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) (netip.Addr, bool) {
	url, found := p.URL[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return netip.Addr{}, false
	}

	ip, ok := getIPFromHTTP(ctx, ppfmt, url)
	if !ok {
		return netip.Addr{}, false
	}

	return ipNet.NormalizeIP(ppfmt, ip)
}
