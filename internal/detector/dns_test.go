package detector_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/detector"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestDNSOverHTTPSName(t *testing.T) {
	t.Parallel()

	policy := &detector.DNSOverHTTPS{
		PolicyName: "very secret name",
		Param:      nil,
	}

	require.Equal(t, "very secret name", detector.Name(policy))
}

func setupServer(t *testing.T, name string, class dnsmessage.Class,
	response bool, header *dnsmessage.Header, idShift uint16, answers []dnsmessage.Resource,
) *httptest.Server {
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

//nolint:funlen,maintidx
func TestDNSOverHTTPSGetIP(t *testing.T) {
	t.Parallel()

	ip4 := net.ParseIP("1.2.3.4").To4()
	ip6 := net.ParseIP("::1:2:3:4").To16()

	for name, tc := range map[string]struct {
		urlKey        ipnet.Type
		ipNet         ipnet.Type
		name          string
		class         dnsmessage.Class
		response      bool
		header        *dnsmessage.Header
		idShift       uint16
		answers       []dnsmessage.Resource
		expected      net.IP
		prepareMockPP func(*mocks.MockPP)
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
			nil,
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
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiError,
					"Failed to prepare the DNS query: %v",
					gomock.Any(),
				)
			},
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
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiError,
					"%q is not a valid %s address",
					ip6,
					"IPv4",
				)
			},
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
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiImpossible, `Invalid DNS response: mismatched transaction ID`,
				)
			},
		},
		"notxt": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustivestruct
			0,
			[]dnsmessage.Resource{},
			nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiImpossible, `Invalid DNS response: no TXT records or all TXT records are empty`,
				)
			},
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
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiImpossible, `Invalid DNS response: QR was not set`,
				)
			},
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
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiImpossible, `Invalid DNS response: TC was set`,
				)
			},
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
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiImpossible,
					"Invalid DNS response: response code is %v",
					dnsmessage.RCodeFormatError,
				)
			},
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
			nil,
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
			nil,
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
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiImpossible, `Invalid DNS response: no TXT records or all TXT records are empty`,
				)
			},
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
			nil,
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
			nil,
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
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiImpossible, `Invalid DNS response: more than one string in TXT records`,
				)
			},
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
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiImpossible, `Invalid DNS response: more than one string in TXT records`,
				)
			},
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
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiImpossible,
					"Invalid DNS response: %v",
					gomock.Any(),
				)
			},
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
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiImpossible,
					"Unhandled IP network: %s",
					"IPv6",
				)
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

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

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ip := policy.GetIP(context.Background(), mockPP, tc.ipNet)
			require.Equal(t, tc.expected, ip)
		})
	}
}
