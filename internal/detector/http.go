package detector

import (
	"context"
	"io"
	"net"
	"net/http"

	"github.com/favonia/cloudflare-ddns-go/internal/pp"
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
