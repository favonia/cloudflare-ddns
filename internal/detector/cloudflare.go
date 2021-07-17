package detector

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

var dnsQuery []byte

func init() {
	msg := dnsmessage.Message{
		Header: dnsmessage.Header{ID: 0x1234},
		Questions: []dnsmessage.Question{
			dnsmessage.Question{
				Name:  dnsmessage.MustNewName("whoami.cloudflare."),
				Type:  dnsmessage.TypeTXT,
				Class: dnsmessage.ClassCHAOS,
			},
		}}
	q, err := msg.Pack()
	if err != nil {
		log.Fatal(err)
	}
	dnsQuery = q
}

func extractIPFromResponse(url string, r []byte) (net.IP, error) {
	var msg dnsmessage.Message
	if err := msg.Unpack(r); err != nil {
		return nil, fmt.Errorf("😩 Not a valid DNS response from %s: %q", url, r)
	}

	if len(msg.Answers) == 0 {
		return nil, fmt.Errorf("😩 Could not find a DNS answer in the response from %s.", url)
	}
	if len(msg.Answers) > 1 {
		return nil, fmt.Errorf("😩 Found more than one DNS answer in the response from %s.", url)
	}

	ans := &msg.Answers[0]
	if ans.Header.Name.String() != "whoami.cloudflare." ||
		ans.Header.Type != dnsmessage.TypeTXT ||
		ans.Header.Class != dnsmessage.ClassCHAOS {
		return nil, fmt.Errorf("🤯 The DNS answer from %s does not match the question: %v", url, ans)
	}

	txt, ok := ans.Body.(*dnsmessage.TXTResource)
	if !ok {
		return nil, fmt.Errorf("🤯 The DNS answer from %s does not match its header: %v", url, ans)
	}

	if len(txt.TXT) == 0 {
		return nil, fmt.Errorf("🤯 The DNS answer from %s is empty: %v", url, ans)
	}
	if len(txt.TXT) > 1 {
		return nil, fmt.Errorf("🤯 The DNS answer from %s has multiple parts: %v", url, ans)
	}

	ip := net.ParseIP(txt.TXT[0])
	if ip == nil {
		return nil, fmt.Errorf(`😩 The DNS answer from %s is not a valid IP address.`, url)
	}

	return ip, nil
}

func getIPFromCloudflare(url string, timeout time.Duration) (net.IP, error) {
	client := http.Client{
		Timeout: timeout,
	}

	resp, err := client.Post(url, "application/dns-message", bytes.NewReader(dnsQuery))
	if err != nil {
		return nil, fmt.Errorf("😩 Could not connect to %s: %v", url, err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("😩 Could not read the response from %s: %v", url, err)
	}

	return extractIPFromResponse(url, b)
}

type Cloudflare struct{}

func (p *Cloudflare) IsManaged() bool {
	return true
}

func (p *Cloudflare) String() string {
	return "cloudflare"
}

func (p *Cloudflare) GetIP4(timeout time.Duration) (net.IP, error) {
	ip, err := getIPFromCloudflare("https://1.1.1.1/dns-query", timeout)
	if err != nil {
		return nil, err
	}
	return ip.To4(), nil
}

func (p *Cloudflare) GetIP6(timeout time.Duration) (net.IP, error) {
	ip, err := getIPFromCloudflare("https://[2606:4700:4700::1111]/dns-query", timeout)
	if err != nil {
		return nil, err
	}
	return ip.To16(), nil
}
