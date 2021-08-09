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

func newDNSQuery(indent pp.Indent, id uint16, name string, class dnsmessage.Class) ([]byte, bool) {
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
		pp.Printf(indent, pp.EmojiError, "Failed to prepare the DNS query: %v", err)

		return nil, false
	}

	return msg, true
}

func parseDNSAnswers(indent pp.Indent, answers []dnsmessage.Resource,
	name string, class dnsmessage.Class) net.IP {
	var ipString string

	for _, ans := range answers {
		if ans.Header.Name.String() != name || ans.Header.Type != dnsmessage.TypeTXT || ans.Header.Class != class {
			continue
		}

		txt, ok := ans.Body.(*dnsmessage.TXTResource)
		if !ok {
			pp.Printf(indent, pp.EmojiImpossible, "The TXT record body is not of type TXTResource: %v", ans)
			return nil
		}

		for _, s := range txt.TXT {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}

			if ipString != "" {
				pp.Printf(indent, pp.EmojiImpossible, "Unexpected multiple non-empty strings in TXT records: %v", answers)
				return nil
			}

			ipString = s
		}
	}

	if ipString == "" {
		pp.Printf(indent, pp.EmojiImpossible, "TXT records have no non-empty strings: %v", answers)
		return nil
	}

	return net.ParseIP(ipString)
}

func parseDNSResponse(indent pp.Indent, r []byte, id uint16, name string, class dnsmessage.Class) net.IP {
	var msg dnsmessage.Message
	if err := msg.Unpack(r); err != nil {
		pp.Printf(indent, pp.EmojiImpossible, "Not a valid DNS response: %v", err)
		return nil
	}

	switch {
	case msg.ID != id:
		pp.Printf(indent, pp.EmojiImpossible, "Response ID %x differs from the query ID %x.", id, msg.ID)
		return nil

	case !msg.Response:
		pp.Printf(indent, pp.EmojiImpossible, "The QR (query/response) bit was not set in the response.")
		return nil

	case msg.Truncated:
		pp.Printf(indent, pp.EmojiImpossible, "The TC (truncation) bit was set. Something went wrong.")
		return nil

	case msg.RCode != dnsmessage.RCodeSuccess:
		pp.Printf(indent, pp.EmojiImpossible, "The response code is %v. The query failed.", msg.RCode)
		return nil
	}

	return parseDNSAnswers(indent, msg.Answers, name, class)
}

func getIPFromDNS(ctx context.Context, indent pp.Indent,
	url string, name string, class dnsmessage.Class) net.IP {
	// message ID for the DNS payloads
	id := randUint16()

	q, ok := newDNSQuery(indent, id, name, class)
	if !ok {
		return nil
	}

	c := httpConn{
		url:         url,
		method:      http.MethodPost,
		contentType: "application/dns-message",
		reader:      bytes.NewReader(q),
		extract: func(indent pp.Indent, body []byte) net.IP {
			return parseDNSResponse(indent, body, id, name, class)
		},
	}

	return c.getIP(ctx, indent)
}

type DNSOverHTTPS struct {
	PolicyName string
	Param      map[ipnet.Type]struct {
		URL   string
		Name  string
		Class dnsmessage.Class
	}
}

func (p *DNSOverHTTPS) IsManaged() bool {
	return true
}

func (p *DNSOverHTTPS) String() string {
	return p.PolicyName
}

func (p *DNSOverHTTPS) GetIP(ctx context.Context, indent pp.Indent, ipNet ipnet.Type) net.IP {
	param, found := p.Param[ipNet]
	if !found {
		return nil
	}

	return ipNet.NormalizeIP(getIPFromDNS(ctx, indent, param.URL, param.Name, param.Class))
}
