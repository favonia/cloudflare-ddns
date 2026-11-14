package protocol

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"net/http"
	"net/netip"
	"strings"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// randUint16 generates a cryptographically secure random uint16.
func randUint16() uint16 {
	var buf [2]byte
	// crypto/rand.Read fills the buffer or terminates the process; it never returns an error.
	_, _ = rand.Read(buf[:])
	return binary.BigEndian.Uint16(buf[:])
}

func newDNSQuery(ppfmt pp.PP, id uint16, name string, class dnsmessage.Class) ([]byte, bool) {
	msg, err := (&dnsmessage.Message{
		Header: dnsmessage.Header{ //nolint:exhaustruct_v5
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
		ppfmt.Noticef(pp.EmojiError, "Failed to prepare the DNS query: %v", err)

		return nil, false
	}

	return msg, true
}

func parseDNSAnswers(ppfmt pp.PP, answers []dnsmessage.Resource, name string, class dnsmessage.Class,
) (netip.Addr, bool) {
	var invalidIP netip.Addr
	var ipString string

	for _, ans := range answers {
		if ans.Header.Name.String() != name || ans.Header.Type != dnsmessage.TypeTXT || ans.Header.Class != class {
			continue
		}

		//nolint:forcetypeassert // The header type check above guarantees TXTResource.
		for _, s := range ans.Body.(*dnsmessage.TXTResource).TXT {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}

			if ipString != "" {
				ppfmt.Noticef(pp.EmojiImpossible, "Invalid DNS response: more than one string in TXT records")
				return invalidIP, false
			}

			ipString = s
		}
	}

	if ipString == "" {
		ppfmt.Noticef(pp.EmojiImpossible, "Invalid DNS response: no TXT records or all TXT records are empty")
		return invalidIP, false
	}

	ip, err := netip.ParseAddr(ipString)
	if err != nil {
		ppfmt.Noticef(
			pp.EmojiImpossible,
			`Invalid DNS response: failed to parse the IP address in the TXT record: %s`,
			ipString,
		)
		return invalidIP, false
	}

	return ip, true
}

func parseDNSResponse(ppfmt pp.PP, r []byte, id uint16, name string, class dnsmessage.Class) (netip.Addr, bool) {
	var invalidIP netip.Addr

	var msg dnsmessage.Message
	if err := msg.Unpack(r); err != nil {
		ppfmt.Noticef(pp.EmojiImpossible, "Invalid DNS response: %v", err)
		return invalidIP, false
	}

	switch {
	case msg.ID != id:
		ppfmt.Noticef(pp.EmojiImpossible, "Invalid DNS response: mismatched transaction ID")
		return invalidIP, false

	case !msg.Response:
		ppfmt.Noticef(pp.EmojiImpossible, "Invalid DNS response: QR was not set")
		return invalidIP, false

	case msg.Truncated:
		ppfmt.Noticef(pp.EmojiImpossible, "Invalid DNS response: TC was set")
		return invalidIP, false

	case msg.RCode != dnsmessage.RCodeSuccess:
		ppfmt.Noticef(pp.EmojiImpossible, "Invalid DNS response: response code is %v", msg.RCode)
		return invalidIP, false
	}

	return parseDNSAnswers(ppfmt, msg.Answers, name, class)
}

func getIPFromDNS(
	ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family,
	url, name string, class dnsmessage.Class,
) (netip.Addr, bool) {
	var invalidIP netip.Addr

	// message ID for the DNS payloads
	id := randUint16()

	q, ok := newDNSQuery(ppfmt, id, name, class)
	if !ok {
		return invalidIP, false
	}

	c := httpCore{ //nolint:exhaustruct_v5 // maxReadLength zero uses the default limit.
		ipFamily: ipFamily,
		url:      url,
		method:   http.MethodPost,
		additionalHeaders: map[string]string{
			"Content-Type": "application/dns-message",
			"Accept":       "application/dns-message",
		},
		requestBody: bytes.NewReader(q),
	}

	body, ok := c.getBody(ctx, ppfmt)
	if !ok {
		return invalidIP, false
	}

	return parseDNSResponse(ppfmt, body, id, name, class)
}

// DNSOverHTTPSParam is the parameter of a DNS-based IP provider.
type DNSOverHTTPSParam = struct {
	URL   string           // the DoH server
	Name  string           // domain name to query
	Class dnsmessage.Class // DNS class to query
}

// DNSOverHTTPS represents a generic detection protocol using DNS over HTTPS.
type DNSOverHTTPS struct {
	ProviderName string // name of the protocol
	Param        map[ipnet.Family]DNSOverHTTPSParam
	// Rejecter is an optional hook called after the IP is parsed from the DNS
	// response. If it rejects the IP, the result is treated as unavailable.
	// This allows provider-specific validation (e.g., rejecting Cloudflare
	// egress IPs) without breaking the generic protocol type.
	Rejecter ipnet.RawIPRejecter
}

// Name of the detection protocol.
func (p DNSOverHTTPS) Name() string {
	return p.ProviderName
}

// IsExplicitEmpty reports whether the provider intentionally clears the family.
func (DNSOverHTTPS) IsExplicitEmpty() bool {
	return false
}

// GetRawData detects the IP address by DNS over HTTPS.
func (p DNSOverHTTPS) GetRawData(
	ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, defaultPrefixLen int,
) DetectionResult {
	param, found := p.Param[ipFamily]
	if !found {
		ppfmt.Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", ipFamily.Describe())
		return NewUnavailableDetectionResult()
	}

	ip, ok := getIPFromDNS(ctx, ppfmt, ipFamily, param.URL, param.Name, param.Class)
	if !ok {
		return NewUnavailableDetectionResult()
	}

	if p.Rejecter != nil {
		if accepted, reason := p.Rejecter.RejectRawIP(ip); !accepted {
			ppfmt.Noticef(pp.EmojiError, "%s", reason)
			return NewUnavailableDetectionResult()
		}
	}

	rawEntries, ok := NormalizeDetectedRawIPs(ppfmt, ipFamily, defaultPrefixLen, []netip.Addr{ip})
	if !ok {
		return NewUnavailableDetectionResult()
	}
	return NewKnownDetectionResult(rawEntries)
}
