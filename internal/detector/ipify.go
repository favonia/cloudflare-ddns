package detector

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

func getIPFromIpify(url string, timeout time.Duration) (net.IP, error) {
	client := http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("😩 Could not connect to %s: %v", url, err)
	}
	defer resp.Body.Close()

	text, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(`😩 Failed to read the response from %s.`, url)
	}

	ip := net.ParseIP(string(text))
	if ip == nil {
		return nil, fmt.Errorf(`😩 Failed to obtain a valid IP address from %s.`, url)
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

func (p *Ipify) GetIP4(timeout time.Duration) (net.IP, error) {
	ip, err := getIPFromIpify("https://api4.ipify.org", timeout)
	if err == nil {
		return ip.To4(), nil
	} else {
		return nil, err
	}
}

func (p *Ipify) GetIP6(timeout time.Duration) (net.IP, error) {
	ip, err := getIPFromIpify("https://api6.ipify.org", timeout)
	if err == nil {
		return ip.To16(), nil
	} else {
		return nil, err
	}
}
