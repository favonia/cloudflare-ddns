package detector_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/detector"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func TestDNSOverHTTPSIsManaged(t *testing.T) {
	t.Parallel()

	policy := detector.DNSOverHTTPS{
		PolicyName: "",
		Param:      nil,
	}

	require.True(t, policy.IsManaged())
}

func TestDNSOverHTTPSString(t *testing.T) {
	t.Parallel()

	policy := detector.DNSOverHTTPS{
		PolicyName: "very secret name",
		Param:      nil,
	}

	require.Equal(t, "very secret name", policy.String())
}

func setupServer(t *testing.T, name string, class dnsmessage.Class,
	response bool, header *dnsmessage.Header, idShift uint16, answers []dnsmessage.Resource) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/dns-message", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/dns-message", r.Header.Get("Accept"))

		var msg dnsmessage.Message
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		err = msg.Unpack(body)
		assert.NoError(t, err)

		assert.Equal(t,
			[]dnsmessage.Question{
				{
					Name:  dnsmessage.MustNewName(name),
					Type:  dnsmessage.TypeTXT,
					Class: class,
				},
			},
			msg.Questions,
		)

		if !response {
			return
		}

		header.ID = msg.ID + idShift

		response, err := (&dnsmessage.Message{
			Header:      *header,
			Questions:   []dnsmessage.Question{},
			Answers:     answers,
			Authorities: []dnsmessage.Resource{},
			Additionals: []dnsmessage.Resource{},
		}).Pack()
		assert.NoError(t, err)

		err = msg.Unpack(response)
		assert.NoError(t, err)

		w.Header().Set("Content-Type", "application/dns-message")
		_, err = w.Write(response)
		assert.NoError(t, err)
	}))
}

//nolint:funlen
func TestDNSOverHTTPSGetIP(t *testing.T) {
	t.Parallel()

	ip4 := net.ParseIP("1.2.3.4").To4()
	ip6 := net.ParseIP("::1:2:3:4").To16()

	for name, tc := range map[string]struct {
		urlKey   ipnet.Type
		ipNet    ipnet.Type
		name     string
		class    dnsmessage.Class
		response bool
		header   *dnsmessage.Header
		idShift  uint16
		answers  []dnsmessage.Resource
		expected net.IP
	}{
		"correct": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			ip4,
		},
		"illformed": {
			ipnet.IP4, ipnet.IP4, "test",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test"),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip6.String()},
					},
				},
			},
			nil,
		},
		"6to4": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip6.String()},
					},
				},
			},
			nil,
		},
		"unmatched-id": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			10,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			nil,
		},
		"notxt": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{},
			nil,
		},
		"notresponse": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			nil,
		},
		"truncated": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true, Truncated: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			nil,
		},
		"rcode": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true, RCode: dnsmessage.RCodeFormatError}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			nil,
		},
		"irrelevant-records1": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassINET,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			ip4,
		},
		"irrelevant-records2": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.AResource{
						A: [4]byte{1, 2, 3, 4},
					},
				},
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			ip4,
		},
		"irrelevant-records3": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test.another."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			nil,
		},
		"irrelevant-records4": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test.another."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			ip4,
		},
		"empty-strings-and-padding": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{"\t", "  " + ip4.String() + "    ", " "},
					},
				},
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{"\t", "     ", " "},
					},
				},
			},
			ip4,
		},
		"multiple1": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String(), ip4.String()},
					},
				},
			},
			nil,
		},
		"multiple2": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			nil,
		},
		"noresponse": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			false,
			&dnsmessage.Header{}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			nil,
		},
		"nourl": {
			ipnet.IP4, ipnet.IP6, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustivestruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			nil,
		},
	} {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := setupServer(t, tc.name, tc.class, tc.response, tc.header, tc.idShift, tc.answers)

			policy := &detector.DNSOverHTTPS{
				PolicyName: "",
				Param: map[ipnet.Type]struct {
					URL   string
					Name  string
					Class dnsmessage.Class
				}{
					tc.urlKey: {server.URL, tc.name, tc.class},
				},
			}

			ip := policy.GetIP(context.Background(), 3, tc.ipNet)
			require.Equal(t, tc.expected, ip)
		})
	}
}
