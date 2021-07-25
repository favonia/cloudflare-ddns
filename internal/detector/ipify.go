package detector

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
)

func getIPFromHTTP(ctx context.Context, url string) (net.IP, bool) {
	// http.Post is avoided so that we can pass ctx
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Printf("😩 Could not generate the request to %s: %v\n", url, err)
		return nil, false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("😩 Could not send the request to %s: %v\n", url, err)
		return nil, false
	}
	defer resp.Body.Close()

	text, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("😩 Failed to read the response from %s.\n", url)
		return nil, false
	}

	ip := net.ParseIP(string(text))
	if ip == nil {
		fmt.Printf("🤯 The response %q is not a valid IP address.\n", text)
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

func (p *Ipify) getIP4(ctx context.Context) (net.IP, bool) {
	ip, ok := getIPFromHTTP(ctx, "https://api4.ipify.org")
	if !ok {
		return nil, false
	}

	return ip.To4(), true
}

func (p *Ipify) getIP6(ctx context.Context) (net.IP, bool) {
	ip, ok := getIPFromHTTP(ctx, "https://api6.ipify.org")
	if !ok {
		return nil, false
	}

	return ip.To16(), true
}

func (p *Ipify) GetIP(ctx context.Context) (net.IP, bool) {
	switch p.Net {
	case ipnet.IP4:
		return p.getIP4(ctx)
	case ipnet.IP6:
		return p.getIP6(ctx)
	default:
		return nil, false
	}
}
