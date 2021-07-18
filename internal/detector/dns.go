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
	return uint16(rand.Uint32())
}

func newDNSQuery(id uint16, name string, class dnsmessage.Class) ([]byte, error) {
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
			dnsmessage.Question{
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
		return nil, fmt.Errorf(`😩 Failed to prepare the DNS query.`)
	}
	return q, nil
}

func parseTXTRecord(r *dnsmessage.TXTResource) (net.IP, error) {
	switch len(r.TXT) {
	case 0:
		return nil, fmt.Errorf("🤯 The TXT record has no strings: %v", r)
	case 1: // good!
	default:
		return nil, fmt.Errorf("🤯 Unexpected multiple strings in the TXT record: %v", r)
	}

	ip := net.ParseIP(r.TXT[0])
	if ip == nil {
		return nil, fmt.Errorf(`🤯 The TXT record %q is not a valid IP address.`, r.TXT[0])
	}

	return ip, nil
}

func parseDNSResource(ans *dnsmessage.Resource, name string, class dnsmessage.Class) (net.IP, error) {
	switch {
	case ans.Header.Name.String() != name:
		return nil, fmt.Errorf("🤯 The DNS answer is for %q, not %q.", ans.Header.Name.String(), name)
	case ans.Header.Type != dnsmessage.TypeTXT:
		return nil, fmt.Errorf("🤯 The DNS answer is of type %v, not %v.", ans.Header.Type, dnsmessage.TypeTXT)
	case ans.Header.Class != class:
		return nil, fmt.Errorf("🤯 The DNS answer is of class %v, not %v.", ans.Header.Class, class)
	}

	txt, ok := ans.Body.(*dnsmessage.TXTResource)
	if !ok {
		return nil, fmt.Errorf("🤯 The TXT record body is not of type TXTResource: %v", ans)
	}

	return parseTXTRecord(txt)
}

func parseDNSResponse(r []byte, id uint16, name string, class dnsmessage.Class) (net.IP, error) {
	var msg dnsmessage.Message
	if err := msg.Unpack(r); err != nil {
		return nil, fmt.Errorf("😩 Not a valid DNS response: %v", err)
	}

	switch {
	case msg.ID != id:
		return nil, fmt.Errorf("😩 Response ID %x differs from the query ID %x.", id, msg.ID)
	case !msg.Response:
		return nil, fmt.Errorf("🤯 The QR (query/response) bit was not set in the response.")
	case msg.Truncated:
		return nil, fmt.Errorf("🤯 The TC (truncation) bit was set. Something went wrong.")
	case msg.RCode != dnsmessage.RCodeSuccess:
		return nil, fmt.Errorf("🤯 The response code is %v. The query failed.", msg.RCode)
	}

	switch len(msg.Answers) {
	case 0:
		return nil, fmt.Errorf("😩 No DNS answers in the response.")
	case 1:
		return parseDNSResource(&msg.Answers[0], name, class)
	default:
		return nil, fmt.Errorf("😩 Unexpected multiple DNS answers in the response.")
	}
}

func getIPFromDNS(ctx context.Context, url string, name string, class dnsmessage.Class) (net.IP, error) {
	id := randUint16()
	q, err := newDNSQuery(id, name, class)
	if err != nil {
		return nil, err
	}

	// http.Post is avoided so that we can pass ctx
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(q))
	if err != nil {
		return nil, fmt.Errorf("😩 Could not generate the request to %s: %v", url, err)
	}

	// set the content type for POST
	req.Header.Set("Content-Type", "application/dns-message")

	// make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("😩 Could not send the request to %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("😩 Failed to read the response from %s: %v", url, err)
	}

	return parseDNSResponse(body, id, name, class)
}
