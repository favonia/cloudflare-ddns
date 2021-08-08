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
	method  string
	url     string
	reader  io.Reader
	prepare func(pp.Indent, *http.Request) bool
	extract func(pp.Indent, []byte) (string, bool)
}

func (d *httpConn) getIP(ctx context.Context, indent pp.Indent) (net.IP, bool) {
	req, err := http.NewRequestWithContext(ctx, d.method, d.url, d.reader)
	if err != nil {
		pp.Printf(indent, pp.EmojiImpossible, "Failed to prepare the request to %q: %v", d.url, err)
		return nil, false
	}

	if ok := d.prepare(indent, req); !ok {
		return nil, false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to send the request to %q: %v", d.url, err)
		return nil, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to read the response from %q: %v", d.url, err)
		return nil, false
	}

	ipString, ok := d.extract(indent, body)
	if !ok {
		return nil, false
	}

	ip := net.ParseIP(ipString)
	if ip == nil {
		pp.Printf(indent, pp.EmojiImpossible, "The response %q is not a valid IP address.", ipString)
		return nil, false
	}

	return ip, true
}

func readIPFromHTTP(ctx context.Context, indent pp.Indent, url string) (net.IP, bool) {
	c := httpConn{
		method:  http.MethodGet,
		url:     url,
		reader:  nil,
		prepare: func(_ pp.Indent, _ *http.Request) bool { return true },
		extract: func(_ pp.Indent, body []byte) (string, bool) { return string(body), true },
	}

	return c.getIP(ctx, indent)
}

type Http struct {
	policyName string
	url        map[ipnet.Type]string
}

func (p *Http) IsManaged() bool {
	return true
}

func (p *Http) String() string {
	return p.policyName
}

func (p *Http) GetIP(ctx context.Context, indent pp.Indent, ipNet ipnet.Type) (net.IP, bool) {
	url, found := p.url[ipNet]
	if !found {
		return nil, false
	}

	ip, ok := readIPFromHTTP(ctx, indent, url)
	if !ok {
		return nil, false
	}

	return ipNet.NormalizeIP(ip), true
}
