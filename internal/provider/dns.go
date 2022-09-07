package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	mathrand "math/rand"
	"net/http"
	"net/netip"
	"strings"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// randUint16 generates a random uint16, possibly not cryptographically secure.
//
//nolint:gosec
func randUint16() uint16 {
	buf := make([]byte, binary.Size(uint16(0)))
	if _, err := rand.Read(buf); err != nil {
		// DoH + a weak PRNG should be secure enough
		return uint16(mathrand.Uint32())
	}

	return binary.BigEndian.Uint16(buf)
}

func newDNSQuery(ppfmt pp.PP, id uint16, name string, class dnsmessage.Class) ([]byte, bool) {
	msg, err := (&dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:               id,
			Response:         false, // query
			OpCode:           0,     // query
			RecursionDesired: false, // no, please

			Authoritative:      false, // meaningless for queries
			Truncated:          false, // meaningless for queries
			RecursionAvailable: false, // meaningless for queries
			RCode:              0,     // meaningless for queries
		},
		Questions: []dnsmessage.Question{
			{
				Name:  dnsmessage.MustNewName(name),
				Type:  dnsmessage.TypeTXT,
				Class: class,
			},
		},
		Answers:     []dnsmessage.Resource{},
		Authorities: []dnsmessage.Resource{},
		Additionals: []dnsmessage.Resource{},
	}).Pack()
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to prepare the DNS query: %v", err)

		return nil, false
	}

	return msg, true
}

func parseDNSAnswers(ppfmt pp.PP, answers []dnsmessage.Resource,
	name string, class dnsmessage.Class,
) netip.Addr {
	var invalidIP netip.Addr
	var ipString string

	for _, ans := range answers {
		if ans.Header.Name.String() != name || ans.Header.Type != dnsmessage.TypeTXT || ans.Header.Class != class {
			continue
		}

		for _, s := range ans.Body.(*dnsmessage.TXTResource).TXT { //nolint:forcetypeassert
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}

			if ipString != "" {
				ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: more than one string in TXT records")
				return invalidIP
			}

			ipString = s
		}
	}

	if ipString == "" {
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: no TXT records or all TXT records are empty")
		return invalidIP
	}

	ip, err := netip.ParseAddr(ipString)
	if err != nil {
		ppfmt.Errorf(
			pp.EmojiImpossible,
			`Invalid DNS response: failed to parse the IP address in the TXT record: %s`,
			ipString,
		)
		return invalidIP
	}
	return ip
}

func parseDNSResponse(ppfmt pp.PP, r []byte, id uint16, name string, class dnsmessage.Class) netip.Addr {
	var invalidIP netip.Addr

	var msg dnsmessage.Message
	if err := msg.Unpack(r); err != nil {
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: %v", err)
		return invalidIP
	}

	switch {
	case msg.ID != id:
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: mismatched transaction ID")
		return invalidIP

	case !msg.Response:
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: QR was not set")
		return invalidIP

	case msg.Truncated:
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: TC was set")
		return invalidIP

	case msg.RCode != dnsmessage.RCodeSuccess:
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: response code is %v", msg.RCode)
		return invalidIP
	}

	return parseDNSAnswers(ppfmt, msg.Answers, name, class)
}

func getIPFromDNS(ctx context.Context, ppfmt pp.PP,
	url string, name string, class dnsmessage.Class,
) netip.Addr {
	var invalidIP netip.Addr

	// message ID for the DNS payloads
	id := randUint16()

	q, ok := newDNSQuery(ppfmt, id, name, class)
	if !ok {
		return invalidIP
	}

	c := httpConn{
		url:         url,
		method:      http.MethodPost,
		contentType: "application/dns-message",
		accept:      "application/dns-message",
		reader:      bytes.NewReader(q),
		extract: func(ppfmt pp.PP, body []byte) netip.Addr {
			return parseDNSResponse(ppfmt, body, id, name, class)
		},
	}

	return c.getIP(ctx, ppfmt)
}

type DNSOverHTTPS struct {
	ProviderName string
	Param        map[ipnet.Type]struct {
		URL   string
		Name  string
		Class dnsmessage.Class
	}
}

func (p *DNSOverHTTPS) Name() string {
	return p.ProviderName
}

func (p *DNSOverHTTPS) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) netip.Addr {
	param, found := p.Param[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return netip.Addr{}
	}

	return NormalizeIP(ppfmt, ipNet, getIPFromDNS(ctx, ppfmt, param.URL, param.Name, param.Class))
}
