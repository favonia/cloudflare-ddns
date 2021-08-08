package detector

import (
	"bytes"
	"context"
	"math/rand"
	"net"
	"net/http"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// randUint16 generates a number using PRNGs, not cryptographically secure.
func randUint16() uint16 {
	return uint16(rand.Uint32()) //nolint:gosec // DNS-over-HTTPS and imperfect pseudorandom should be more than enough
}

func newDNSQuery(indent pp.Indent, id uint16, name string, class dnsmessage.Class) ([]byte, bool) {
	msg := dnsmessage.Message{
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
	}

	q, err := msg.Pack()
	if err != nil {
		pp.Printf(indent, pp.EmojiError, "Failed to prepare the DNS query: %v", err)

		return nil, false
	}

	return q, true
}

func parseTXTRecord(indent pp.Indent, r *dnsmessage.TXTResource) (string, bool) {
	switch len(r.TXT) {
	case 0: // len(r.TXT) == 0
		pp.Printf(indent, pp.EmojiImpossible, "The TXT record has no strings: %v", r)
		return "", false

	case 1: // len(r.TXT) == 1
		break

	default: // len(r.TXT) > 1
		pp.Printf(indent, pp.EmojiImpossible, "Unexpected multiple strings in the TXT record: %v", r)
		return "", false
	}

	return r.TXT[0], true
}

func parseDNSResource(indent pp.Indent, ans *dnsmessage.Resource, name string, class dnsmessage.Class) (string, bool) {
	switch {
	case ans.Header.Name.String() != name:
		pp.Printf(indent, pp.EmojiImpossible, "The DNS answer is for %q, not %q.", ans.Header.Name.String(), name)
		return "", false
	case ans.Header.Type != dnsmessage.TypeTXT:
		pp.Printf(indent, pp.EmojiImpossible, "The DNS answer is of type %v, not %v.", ans.Header.Type, dnsmessage.TypeTXT)
		return "", false
	case ans.Header.Class != class:
		pp.Printf(indent, pp.EmojiImpossible, "The DNS answer is of class %v, not %v.", ans.Header.Class, class)
		return "", false
	}

	txt, ok := ans.Body.(*dnsmessage.TXTResource)
	if !ok {
		pp.Printf(indent, pp.EmojiImpossible, "The TXT record body is not of type TXTResource: %v", ans)
		return "", false
	}

	return parseTXTRecord(indent, txt)
}

func parseDNSResponse(indent pp.Indent, r []byte, id uint16, name string, class dnsmessage.Class) (string, bool) {
	var msg dnsmessage.Message
	if err := msg.Unpack(r); err != nil {
		pp.Printf(indent, pp.EmojiImpossible, "Not a valid DNS response: %v", err)
		return "", false
	}

	switch {
	case msg.ID != id:
		pp.Printf(indent, pp.EmojiImpossible, "Response ID %x differs from the query ID %x.", id, msg.ID)
		return "", false

	case !msg.Response:
		pp.Printf(indent, pp.EmojiImpossible, "The QR (query/response) bit was not set in the response.")
		return "", false

	case msg.Truncated:
		pp.Printf(indent, pp.EmojiImpossible, "The TC (truncation) bit was set. Something went wrong.")
		return "", false

	case msg.RCode != dnsmessage.RCodeSuccess:
		pp.Printf(indent, pp.EmojiImpossible, "The response code is %v. The query failed.", msg.RCode)
		return "", false
	}

	switch len(msg.Answers) {
	case 0: // len(msg.Answers) == 0
		pp.Printf(indent, pp.EmojiImpossible, "No DNS answers in the response.")
		return "", false
	case 1: // len(msg.Answers) == 1
		return parseDNSResource(indent, &msg.Answers[0], name, class)
	default: // len(msg.Answers) > 1
		pp.Printf(indent, pp.EmojiImpossible, "Unexpected multiple DNS answers in the response.")
		return "", false
	}
}

func getIPFromDNS(ctx context.Context, indent pp.Indent,
	url string, name string, class dnsmessage.Class) (net.IP, bool) {
	// message ID for the DNS payloads
	id := randUint16()

	q, ok := newDNSQuery(indent, id, name, class)
	if !ok {
		return nil, false
	}

	c := httpConn{
		method: http.MethodPost,
		url:    url,
		reader: bytes.NewReader(q),
		prepare: func(_ pp.Indent, req *http.Request) bool {
			req.Header.Set("Content-Type", "application/dns-message")
			return true
		},
		extract: func(ident pp.Indent, body []byte) (string, bool) {
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

func (p *DNSOverHTTPS) GetIP(ctx context.Context, indent pp.Indent, ipNet ipnet.Type) (net.IP, bool) {
	param, found := p.Param[ipNet]
	if !found {
		return nil, false
	}

	ip, ok := getIPFromDNS(ctx, indent, param.URL, param.Name, param.Class)
	if !ok || ip == nil {
		return nil, false
	}

	return ipNet.NormalizeIP(ip), true
}
