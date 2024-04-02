package protocol_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestDNSOverHTTPSName(t *testing.T) {
	t.Parallel()

	p := &protocol.DNSOverHTTPS{
		ProviderName:     "very secret name",
		Is1111UsedForIP4: false,
		Param:            nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

func setupServer(t *testing.T, name string, class dnsmessage.Class,
	response bool, header *dnsmessage.Header, idShift uint16, answers []dnsmessage.Resource,
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/dns-message", r.Header.Get("Content-Type"))
		require.Equal(t, "application/dns-message", r.Header.Get("Accept"))

		var msg dnsmessage.Message
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		err = msg.Unpack(body)
		require.NoError(t, err)

		require.Equal(t,
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
		require.NoError(t, err)

		err = msg.Unpack(response)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/dns-message")
		_, err = w.Write(response)
		require.NoError(t, err)
	}))
}

//nolint:funlen
func TestDNSOverHTTPSGetIP(t *testing.T) {
	t.Parallel()

	ip4 := netip.MustParseAddr("1.2.3.4")
	ip6 := netip.MustParseAddr("2606:4700:4700::1234")
	invalidIP := netip.Addr{}

	for name, tc := range map[string]struct {
		urlKey        ipnet.Type
		ipNet         ipnet.Type
		name          string
		class         dnsmessage.Class
		response      bool
		header        *dnsmessage.Header
		idShift       uint16
		answers       []dnsmessage.Resource
		expected      netip.Addr
		prepareMockPP func(*mocks.MockPP)
	}{
		"correct": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
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
		"illformed-query": {
			ipnet.IP4, ipnet.IP4, "test",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test"),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip6.String()},
					},
				},
			},
			invalidIP,
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
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip6.String()},
					},
				},
			},
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiError,
					"Detected IP address %s is not a valid %s address",
					ip6.String(),
					"IPv4",
				)
			},
		},
		"unmatched-id": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			10,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			invalidIP,
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
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{},
			invalidIP,
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
			&dnsmessage.Header{}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			invalidIP,
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
			&dnsmessage.Header{Response: true, Truncated: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			invalidIP,
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
			&dnsmessage.Header{Response: true, RCode: dnsmessage.RCodeFormatError}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			invalidIP,
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
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassINET,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
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
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.AResource{
						A: [4]byte{1, 2, 3, 4},
					},
				},
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
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
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test.another."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			invalidIP,
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
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
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
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{"\t", "  " + ip4.String() + "    ", " "},
					},
				},
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
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
		"illformed-ip": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{"I am definitely not an IP address"},
					},
				},
			},
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(
					pp.EmojiImpossible,
					`Invalid DNS response: failed to parse the IP address in the TXT record: %s`,
					"I am definitely not an IP address",
				)
			},
		},
		"multiple1": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String(), ip4.String()},
					},
				},
			},
			invalidIP,
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
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			invalidIP,
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
			&dnsmessage.Header{}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			invalidIP,
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
			&dnsmessage.Header{}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{
				{
					Header: dnsmessage.ResourceHeader{ //nolint:exhaustruct
						Name:  dnsmessage.MustNewName("test."),
						Class: dnsmessage.ClassCHAOS,
					},
					Body: &dnsmessage.TXTResource{
						TXT: []string{ip4.String()},
					},
				},
			},
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiImpossible,
					"Unhandled IP network: %s",
					"IPv6",
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			server := setupServer(t, tc.name, tc.class, tc.response, tc.header, tc.idShift, tc.answers)

			provider := &protocol.DNSOverHTTPS{
				ProviderName:     "",
				Is1111UsedForIP4: false,
				Param: map[ipnet.Type]struct {
					URL   protocol.Switch
					Name  string
					Class dnsmessage.Class
				}{
					tc.urlKey: {protocol.Constant(server.URL), tc.name, tc.class},
				},
			}

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ip, ok := provider.GetIP(context.Background(), mockPP, tc.ipNet, true)
			require.Equal(t, tc.expected, ip)
			require.Equal(t, tc.expected.IsValid(), ok)
		})
	}
}

func TestDOHShouldWeCheck1111(t *testing.T) {
	t.Parallel()

	require.True(t, (&protocol.DNSOverHTTPS{
		ProviderName:     "",
		Is1111UsedForIP4: true,
		Param:            nil,
	}).ShouldWeCheck1111())

	require.False(t, (&protocol.DNSOverHTTPS{
		ProviderName:     "",
		Is1111UsedForIP4: false,
		Param:            nil,
	}).ShouldWeCheck1111())
}
