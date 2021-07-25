package detector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"

	"golang.org/x/net/dns/dnsmessage"
)

// randUint16 generates a number using PRNGs, not cryptographically secure.
func randUint16() uint16 {
	return uint16(rand.Uint32()) //nolint:gosec // DNS-over-HTTPS and imperfect pseudorandom should be more than enough
}

func newDNSQuery(id uint16, name string, class dnsmessage.Class) ([]byte, bool) {
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
		fmt.Printf("😩 Failed to prepare the DNS query: %v\n", err)

		return nil, false
	}

	return q, true
}

func parseTXTRecord(r *dnsmessage.TXTResource) (net.IP, bool) {
	switch len(r.TXT) {
	case 0: // len(r.TXT) == 0
		fmt.Printf("🤯 The TXT record has no strings: %v\n", r)
		return nil, false

	case 1: // len(r.TXT) == 1
		break

	default: // len(r.TXT) > 1
		fmt.Printf("🤯 Unexpected multiple strings in the TXT record: %v\n", r)
		return nil, false
	}

	ip := net.ParseIP(r.TXT[0])
	if ip == nil {
		fmt.Printf("🤯 The TXT record %q is not a valid IP address.\n", r.TXT[0])
		return nil, false
	}

	return ip, true
}

func parseDNSResource(ans *dnsmessage.Resource, name string, class dnsmessage.Class) (net.IP, bool) {
	switch {
	case ans.Header.Name.String() != name:
		fmt.Printf("🤯 The DNS answer is for %q, not %q.\n", ans.Header.Name.String(), name)
		return nil, false
	case ans.Header.Type != dnsmessage.TypeTXT:
		fmt.Printf("🤯 The DNS answer is of type %v, not %v.\n", ans.Header.Type, dnsmessage.TypeTXT)
		return nil, false
	case ans.Header.Class != class:
		fmt.Printf("🤯 The DNS answer is of class %v, not %v.\n", ans.Header.Class, class)
		return nil, false
	}

	txt, ok := ans.Body.(*dnsmessage.TXTResource)
	if !ok {
		fmt.Printf("🤯 The TXT record body is not of type TXTResource: %v\n", ans)
		return nil, false
	}

	return parseTXTRecord(txt)
}

func parseDNSResponse(r []byte, id uint16, name string, class dnsmessage.Class) (net.IP, bool) {
	var msg dnsmessage.Message
	if err := msg.Unpack(r); err != nil {
		fmt.Printf("😩 Not a valid DNS response: %v\n", err)
		return nil, false
	}

	switch {
	case msg.ID != id:
		fmt.Printf("😩 Response ID %x differs from the query ID %x.\n", id, msg.ID)
		return nil, false

	case !msg.Response:
		fmt.Printf("🤯 The QR (query/response) bit was not set in the response.\n")
		return nil, false

	case msg.Truncated:
		fmt.Printf("🤯 The TC (truncation) bit was set. Something went wrong.\n")
		return nil, false

	case msg.RCode != dnsmessage.RCodeSuccess:
		fmt.Printf("🤯 The response code is %v. The query failed.\n", msg.RCode)
		return nil, false
	}

	switch len(msg.Answers) {
	case 0: // len(msg.Answers) == 0
		fmt.Printf("😩 No DNS answers in the response.\n")
		return nil, false
	case 1: // len(msg.Answers) == 1
		return parseDNSResource(&msg.Answers[0], name, class)
	default: // len(msg.Answers) > 1
		fmt.Printf("😩 Unexpected multiple DNS answers in the response.\n")
		return nil, false
	}
}

func getIPFromDNS(ctx context.Context, url string, name string, class dnsmessage.Class) (net.IP, bool) {
	// message ID for the DNS payloads
	id := randUint16()

	q, ok := newDNSQuery(id, name, class)
	if !ok {
		return nil, false
	}

	// http.Post is avoided so that we can pass ctx
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(q))
	if err != nil {
		fmt.Printf("😩 Could not generate the request to %s: %v\n", url, err)
		return nil, false
	}

	// set the content type for POST
	req.Header.Set("Content-Type", "application/dns-message")

	// make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("😩 Could not send the request to %s: %v\n", url, err)
		return nil, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("😩 Failed to read the response from %s: %v\n", url, err)
		return nil, false
	}

	return parseDNSResponse(body, id, name, class)
}
