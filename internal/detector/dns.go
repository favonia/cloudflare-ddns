package detector

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	mathrand "math/rand"
	"net"
	"net/http"
	"strings"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// randUint16 generates a random uint16, possibly not cryptographically secure.
func randUint16() uint16 {
	buf := make([]byte, binary.Size(uint16(0)))
	if _, err := rand.Read(buf); err != nil {
		// DoH + a weak PRNG should be secure enough
		return uint16(mathrand.Uint32()) //nolint:gosec
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
	name string, class dnsmessage.Class) net.IP {
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
				return nil
			}

			ipString = s
		}
	}

	if ipString == "" {
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: no TXT records or all TXT records are empty")
		return nil
	}

	return net.ParseIP(ipString)
}

func parseDNSResponse(ppfmt pp.PP, r []byte, id uint16, name string, class dnsmessage.Class) net.IP {
	var msg dnsmessage.Message
	if err := msg.Unpack(r); err != nil {
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: %v", err)
		return nil
	}

	switch {
	case msg.ID != id:
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: mismatched transaction ID")
		return nil

	case !msg.Response:
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: QR was not set")
		return nil

	case msg.Truncated:
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: TC was set")
		return nil

	case msg.RCode != dnsmessage.RCodeSuccess:
		ppfmt.Warningf(pp.EmojiImpossible, "Invalid DNS response: response code is %v", msg.RCode)
		return nil
	}

	return parseDNSAnswers(ppfmt, msg.Answers, name, class)
}

func getIPFromDNS(ctx context.Context, ppfmt pp.PP,
	url string, name string, class dnsmessage.Class) net.IP {
	// message ID for the DNS payloads
	id := randUint16()

	q, ok := newDNSQuery(ppfmt, id, name, class)
	if !ok {
		return nil
	}

	c := httpConn{
		url:         url,
		method:      http.MethodPost,
		contentType: "application/dns-message",
		accept:      "application/dns-message",
		reader:      bytes.NewReader(q),
		extract: func(ppfmt pp.PP, body []byte) net.IP {
			return parseDNSResponse(ppfmt, body, id, name, class)
		},
	}

	return c.getIP(ctx, ppfmt)
}

type DNSOverHTTPS struct {
	PolicyName string
	Param      map[ipnet.Type]struct {
		URL   string
		Name  string
		Class dnsmessage.Class
	}
}

func (p *DNSOverHTTPS) name() string {
	return p.PolicyName
}

func (p *DNSOverHTTPS) GetIP(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type) net.IP {
	param, found := p.Param[ipNet]
	if !found {
		ppfmt.Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", ipNet.Describe())
		return nil
	}

	return NormalizeIP(ppfmt, ipNet, getIPFromDNS(ctx, ppfmt, param.URL, param.Name, param.Class))
}
