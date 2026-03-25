package protocol_test

// vim: nowrap

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/net/dns/dnsmessage"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// testIPRejecter is a test helper that always rejects with a message containing the IP.
type testIPRejecter struct{}

func (testIPRejecter) RejectRawIP(ip netip.Addr) (bool, string) {
	return false, "rejected: " + ip.String()
}

// testIPAccepter is a test helper that always accepts.
type testIPAccepter struct{}

func (testIPAccepter) RejectRawIP(_ netip.Addr) (bool, string) {
	return true, ""
}

func TestDNSOverHTTPSName(t *testing.T) {
	t.Parallel()

	p := protocol.DNSOverHTTPS{ //nolint:exhaustruct // only testing Name()
		ProviderName: "very secret name",
		Param:        nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

func setupServer(t *testing.T, name string, class dnsmessage.Class,
	response bool, header *dnsmessage.Header, idShift uint16, answers []dnsmessage.Resource,
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, http.MethodPost, r.Method) ||
			!assert.Equal(t, "application/dns-message", r.Header.Get("Content-Type")) ||
			!assert.Equal(t, "application/dns-message", r.Header.Get("Accept")) {
			panic(http.ErrAbortHandler)
		}

		var msg dnsmessage.Message
		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			panic(http.ErrAbortHandler)
		}

		if err := msg.Unpack(body); !assert.NoError(t, err) {
			panic(http.ErrAbortHandler)
		}

		if !assert.Equal(t,
			[]dnsmessage.Question{
				{
					Name:  dnsmessage.MustNewName(name),
					Type:  dnsmessage.TypeTXT,
					Class: class,
				},
			},
			msg.Questions,
		) {
			panic(http.ErrAbortHandler)
		}

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
		if !assert.NoError(t, err) {
			panic(http.ErrAbortHandler)
		}

		if err := msg.Unpack(response); !assert.NoError(t, err) {
			panic(http.ErrAbortHandler)
		}

		w.Header().Set("Content-Type", "application/dns-message")
		if _, err = w.Write(response); !assert.NoError(t, err) {
			panic(http.ErrAbortHandler)
		}
	}))
}

func TestDNSOverHTTPSGetRawData(t *testing.T) {
	t.Parallel()

	ip4 := netip.MustParseAddr("1.2.3.4")
	ip6 := netip.MustParseAddr("2606:4700:4700::1234")
	invalidIP := netip.Addr{}

	for name, tc := range map[string]struct {
		urlKey        ipnet.Family
		ipFamily      ipnet.Family
		name          string
		class         dnsmessage.Class
		response      bool
		header        *dnsmessage.Header
		idShift       uint16
		answers       []dnsmessage.Resource
		rejecter      ipnet.RawIPRejecter
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
			nil,
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to prepare the DNS query: %v", gomock.Any())
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s %s", ip6.String(), "is not a valid IPv4 address")
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Invalid DNS response: mismatched transaction ID`)
			},
		},
		"notxt": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true}, //nolint:exhaustruct
			0,
			[]dnsmessage.Resource{},
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Invalid DNS response: no TXT records or all TXT records are empty`)
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Invalid DNS response: QR was not set`)
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Invalid DNS response: TC was set`)
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Invalid DNS response: response code is %v", dnsmessage.RCodeFormatError)
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
			nil,
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
			nil,
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Invalid DNS response: no TXT records or all TXT records are empty`)
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
			nil,
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
			nil,
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Invalid DNS response: failed to parse the IP address in the TXT record: %s`, "I am definitely not an IP address")
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Invalid DNS response: more than one string in TXT records`)
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Invalid DNS response: more than one string in TXT records`)
			},
		},
		"reject-ip": {
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
			testIPRejecter{},
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "%s", "rejected: "+ip4.String())
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Invalid DNS response: %v", gomock.Any())
			},
		},
		"inet-class": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassINET,
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
			},
			nil,
			ip4,
			nil,
		},
		"accept-ip": {
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
			testIPAccepter{},
			ip4,
			nil,
		},
		"rcode-server-failure": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true, RCode: dnsmessage.RCodeServerFailure}, //nolint:exhaustruct
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Invalid DNS response: response code is %v", dnsmessage.RCodeServerFailure)
			},
		},
		"rcode-name-error": {
			ipnet.IP4, ipnet.IP4, "test.",
			dnsmessage.ClassCHAOS,
			true,
			&dnsmessage.Header{Response: true, RCode: dnsmessage.RCodeNameError}, //nolint:exhaustruct
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Invalid DNS response: response code is %v", dnsmessage.RCodeNameError)
			},
		},
		"all-empty-txt-strings": {
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
						TXT: []string{"", "   ", "\t"},
					},
				},
			},
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, `Invalid DNS response: no TXT records or all TXT records are empty`)
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
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", "IPv6")
			},
		},
		"nourl-ip6": {
			ipnet.IP6, ipnet.IP4, "test.",
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
						TXT: []string{ip6.String()},
					},
				},
			},
			nil,
			invalidIP,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", "IPv4")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)

			server := setupServer(t, tc.name, tc.class, tc.response, tc.header, tc.idShift, tc.answers)

			provider := &protocol.DNSOverHTTPS{
				ProviderName: "",
				Param: map[ipnet.Family]protocol.DNSOverHTTPSParam{
					tc.urlKey: {server.URL, tc.name, tc.class},
				},
				Rejecter: tc.rejecter,
			}

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			rawData := provider.GetRawData(context.Background(), mockPP, tc.ipFamily, map[ipnet.Family]int{
				ipnet.IP4: 32,
				ipnet.IP6: 64,
			}[tc.ipFamily])
			require.Equal(t, tc.expected.IsValid(), rawData.Available)
			if tc.expected.IsValid() {
				require.Equal(t, []ipnet.RawEntry{ipnet.RawEntryFrom(tc.expected, map[ipnet.Family]int{
					ipnet.IP4: 32,
					ipnet.IP6: 64,
				}[tc.ipFamily])}, rawData.RawEntries)
			} else {
				require.Empty(t, rawData.RawEntries)
			}
		})
	}
}

func TestDNSOverHTTPSNameIsExplicitEmpty(t *testing.T) {
	t.Parallel()

	require.False(t, protocol.DNSOverHTTPS{ //nolint:exhaustruct // only testing IsExplicitEmpty()
		ProviderName: "",
		Param: map[ipnet.Family]protocol.DNSOverHTTPSParam{
			ipnet.IP4: {"https://localhost", "hello.", dnsmessage.ClassCHAOS},
		},
	}.IsExplicitEmpty())
}
