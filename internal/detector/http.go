package detector

import (
	"context"
	"io"
	"net"
	"net/http"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type httpConn struct {
	url         string
	method      string
	contentType string
	accept      string
	reader      io.Reader
	extract     func(pp.Fmt, []byte) net.IP
}

func (d *httpConn) getIP(ctx context.Context, ppfmt pp.Fmt) net.IP {
	req, err := http.NewRequestWithContext(ctx, d.method, d.url, d.reader)
	if err != nil {
		ppfmt.Warningf(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v", d.url, err)
		return nil
	}

	if d.contentType != "" {
		req.Header.Set("Content-Type", d.contentType)
	}

	if d.accept != "" {
		req.Header.Set("Accept", d.accept)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", d.url, err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to read HTTP(S) response from %q: %v", d.url, err)
		return nil
	}

	return d.extract(ppfmt, body)
}

func getIPFromHTTP(ctx context.Context, ppfmt pp.Fmt, url string) net.IP {
	c := httpConn{
		url:         url,
		method:      http.MethodGet,
		contentType: "",
		accept:      "",
		reader:      nil,
		extract:     func(_ pp.Fmt, body []byte) net.IP { return net.ParseIP(string(body)) },
	}

	return c.getIP(ctx, ppfmt)
}

type HTTP struct {
	PolicyName string
	URL        map[ipnet.Type]string
}

func (p *HTTP) IsManaged() bool {
	return true
}

func (p *HTTP) String() string {
	return p.PolicyName
}

func (p *HTTP) GetIP(ctx context.Context, ppfmt pp.Fmt, ipNet ipnet.Type) net.IP {
	url, found := p.URL[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return nil
	}

	return NormalizeIP(ppfmt, ipNet, getIPFromHTTP(ctx, ppfmt, url))
}
