package detector

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
)

func getIPFromIpify(ctx context.Context, url string) (net.IP, error) {
	// http.Post is avoided so that we can pass ctx
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("😩 Could not generate the request to %s: %v", url, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("😩 Could not send the request to %s: %v", url, err)
	}
	defer resp.Body.Close()

	text, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(`😩 Failed to read the response from %s.`, url)
	}

	ip := net.ParseIP(string(text))
	if ip == nil {
		return nil, fmt.Errorf(`🤯 The response %q is not a valid IP address.`, text)
	}

	return ip, nil
}

type Ipify struct{}

func (p *Ipify) IsManaged() bool {
	return true
}

func (p *Ipify) String() string {
	return "ipify"
}

func (p *Ipify) GetIP4(ctx context.Context) (net.IP, error) {
	ip, err := getIPFromIpify(ctx, "https://api4.ipify.org")
	if err != nil {
		return nil, err
	}
	return ip.To4(), nil
}

func (p *Ipify) GetIP6(ctx context.Context) (net.IP, error) {
	ip, err := getIPFromIpify(ctx, "https://api6.ipify.org")
	if err != nil {
		return nil, err
	}
	return ip.To16(), nil
}
