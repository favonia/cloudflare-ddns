package detector

import (
	"context"
	"io"
	"net"
	"net/http"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

func getIPFromHTTP(ctx context.Context, indent pp.Indent, url string) (net.IP, bool) {
	// http.Post is avoided so that we can pass ctx
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		pp.Printf(indent, pp.EmojiImpossible, "Failed to prepare the HTTP request to %q: %v", url, err)
		return nil, false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to send the request to %s: %v\n", url, err)
		return nil, false
	}
	defer resp.Body.Close()

	text, err := io.ReadAll(resp.Body)
	if err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to read the response from %s.\n", url)
		return nil, false
	}

	ip := net.ParseIP(string(text))
	if ip == nil {
		pp.Printf(indent, pp.EmojiImpossible, "The response %q is not a valid IP address.\n", text)
		return nil, false
	}

	return ip, true
}

type Ipify struct {
	Net ipnet.Type
}

func (p *Ipify) IsManaged() bool {
	return true
}

func (p *Ipify) String() string {
	return "ipify"
}

func (p *Ipify) getIP4(ctx context.Context, indent pp.Indent) (net.IP, bool) {
	ip, ok := getIPFromHTTP(ctx, indent, "https://api4.ipify.org")
	if !ok {
		return nil, false
	}

	return ip.To4(), true
}

func (p *Ipify) getIP6(ctx context.Context, indent pp.Indent) (net.IP, bool) {
	ip, ok := getIPFromHTTP(ctx, indent, "https://api6.ipify.org")
	if !ok {
		return nil, false
	}

	return ip.To16(), true
}

func (p *Ipify) GetIP(ctx context.Context, indent pp.Indent) (net.IP, bool) {
	switch p.Net {
	case ipnet.IP4:
		return p.getIP4(ctx, indent)
	case ipnet.IP6:
		return p.getIP6(ctx, indent)
	default:
		return nil, false
	}
}
