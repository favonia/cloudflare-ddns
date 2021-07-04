package detector

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/common"
)

func getIPFromCloudflare(url string) (net.IP, error) {
	timeout := time.Second * 5
	client := http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("😡 Could not connect to the CloudFlare server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(?m:^ip=(.*)$)`)
	ms := re.FindSubmatch(body)
	if ms == nil {
		return nil, fmt.Errorf(`😡 Could not find "ip=..." in the response: %q.`, string(body))
	}

	return net.ParseIP(string(ms[1])), nil
}

func getIP4FromCloadflare() (net.IP, error) {
	ip, err := getIPFromCloudflare("https://1.1.1.1/cdn-cgi/trace")
	if err == nil {
		return ip.To4(), nil
	} else {
		return nil, err
	}
}

func getIP6FromCloadflare() (net.IP, error) {
	ip, err := getIPFromCloudflare("https://[2606:4700:4700::1111]/cdn-cgi/trace")
	if err == nil {
		return ip.To16(), nil
	} else {
		return nil, err
	}
}

func getLocalIP4() (net.IP, error) {
	conn, err := net.Dial("udp4", "1.1.1.1:443")
	if err != nil {
		return nil, fmt.Errorf(`😡 Could not detect a local IPv4 address: %w`, err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.To4(), nil
}

func getLocalIP6() (net.IP, error) {
	conn, err := net.Dial("udp6", "[2606:4700:4700::1111]:443")
	if err != nil {
		return nil, fmt.Errorf(`😡 Could not detect a local IPv6 address: %w`, err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.To16(), nil
}

func GetIP4(policy common.Policy) (net.IP, error) {
	switch policy {
	case common.Cloudflare:
		return getIP4FromCloadflare()
	case common.Local:
		return getLocalIP4()
	default:
		return nil, fmt.Errorf("😱 The impossible happened!")
	}
}

func GetIP6(policy common.Policy) (net.IP, error) {
	switch policy {
	case common.Cloudflare:
		return getIP6FromCloadflare()
	case common.Local:
		return getLocalIP6()
	default:
		return nil, fmt.Errorf("😱 The impossible happened!")
	}
}
