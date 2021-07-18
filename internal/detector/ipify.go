package detector

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
)

func getIPFromIpify(ctx context.Context, url string) (net.IP, bool) {
	// http.Post is avoided so that we can pass ctx
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("😩 Could not generate the request to %s: %v", url, err)
		return nil, false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("😩 Could not send the request to %s: %v", url, err)
		return nil, false
	}
	defer resp.Body.Close()

	text, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf(`😩 Failed to read the response from %s.`, url)
		return nil, false
	}

	ip := net.ParseIP(string(text))
	if ip == nil {
		log.Printf(`🤯 The response %q is not a valid IP address.`, text)
		return nil, false
	}

	return ip, true
}

type Ipify struct{}

func (p *Ipify) IsManaged() bool {
	return true
}

func (p *Ipify) String() string {
	return "ipify"
}

func (p *Ipify) GetIP4(ctx context.Context) (net.IP, bool) {
	ip, ok := getIPFromIpify(ctx, "https://api4.ipify.org")
	if !ok {
		return nil, false
	}
	return ip.To4(), true
}

func (p *Ipify) GetIP6(ctx context.Context) (net.IP, bool) {
	ip, ok := getIPFromIpify(ctx, "https://api6.ipify.org")
	if !ok {
		return nil, false
	}
	return ip.To16(), true
}
