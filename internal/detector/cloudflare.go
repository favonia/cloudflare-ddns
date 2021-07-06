package detector

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"time"
)

func getIPFromCloudflare(url string, timeout time.Duration) (net.IP, error) {
	client := http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("😩 Could not connect to %s: %v", url, err)
	}
	defer resp.Body.Close()

	// we need `buf` because ``re.FindReaderSubmatch
	var buf bytes.Buffer
	teeReader := io.TeeReader(resp.Body, &buf)
	teeRuneReader := bufio.NewReader(teeReader)

	re := regexp.MustCompile(`(?m:^ip=(.*)$)`)
	loc := re.FindReaderSubmatchIndex(teeRuneReader)
	if loc == nil {
		return nil, fmt.Errorf(`😩 Failed to find "ip=..." in the response of %s.`, url)
	}
	matched := string(bytes.Runes(buf.Bytes())[loc[2]:loc[3]])

	ip := net.ParseIP(matched)
	if ip == nil {
		return nil, fmt.Errorf(`😩 Failed to obtain a valid IP address from %s.`, url)
	}

	return ip, nil
}

type Cloudflare struct{}

func (p *Cloudflare) IsManaged() bool {
	return true
}

func (p *Cloudflare) String() string {
	return "cloudflare"
}

func (p *Cloudflare) GetIP4(timeout time.Duration) (net.IP, error) {
	ip, err := getIPFromCloudflare("https://1.1.1.1/cdn-cgi/trace", timeout)
	if err == nil {
		return ip.To4(), nil
	} else {
		return nil, err
	}
}

func (p *Cloudflare) GetIP6(timeout time.Duration) (net.IP, error) {
	ip, err := getIPFromCloudflare("https://[2606:4700:4700::1111]/cdn-cgi/trace", timeout)
	if err == nil {
		return ip.To16(), nil
	} else {
		return nil, err
	}
}
