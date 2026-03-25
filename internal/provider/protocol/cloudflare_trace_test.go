package protocol_test

// vim: nowrap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestCloudflareTraceName(t *testing.T) {
	t.Parallel()

	p := protocol.CloudflareTrace{
		ProviderName: "very secret name",
		URL:          nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

func TestCloudflareTraceIsExplicitEmpty(t *testing.T) {
	t.Parallel()

	require.False(t, protocol.CloudflareTrace{
		ProviderName: "",
		URL:          nil,
	}.IsExplicitEmpty())
}

// hostFromURL extracts the Host field from a URL string for use in trace h= fields.
func hostFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

func TestCloudflareTraceGetRawData(t *testing.T) {
	t.Parallel()

	ip4 := netip.MustParseAddr("1.2.3.4")
	ip6 := netip.MustParseAddr("::1:2:3:4:5:6")

	type testCase struct {
		ipFamily      ipnet.Family
		serverFamily  ipnet.Family
		makeResponse  func(serverURL string) string
		noServer      bool   // skip creating a test server
		forceURL      string // override URL for the provider
		unmappedIP    ipnet.Family
		available     bool
		expected      netip.Addr
		prepareMockPP func(serverURL string, m *mocks.MockPP)
	}

	for name, tc := range map[string]testCase{
		"4/valid": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=off\n", hostFromURL(serverURL), ip4)
			},
			available: true, expected: ip4,
		},
		"6/valid": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=off\n", hostFromURL(serverURL), ip6)
			},
			available: true, expected: ip6,
		},
		"4/missing-h-warns": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(_ string) string {
				return fmt.Sprintf("ip=%s\nwarp=off\n", ip4)
			},
			available: true, expected: ip4,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible,
					"The response of %q does not contain an h (host) field; please report this at %s",
					gomock.Any(), pp.IssueReportingURL)
			},
		},
		"4/mismatched-h": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(_ string) string {
				return fmt.Sprintf("h=wrong.example.com\nip=%s\nwarp=off\n", ip4)
			},
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible,
					"The h field %q in the response of %q does not match the expected host %q; please report this at %s",
					"wrong.example.com", gomock.Any(), gomock.Any(), pp.IssueReportingURL)
			},
		},
		"4/warp-on": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=on\n", hostFromURL(serverURL), ip4)
			},
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"The response of %q has warp=on; the detected IP is a Cloudflare WARP egress IP, not your real public IP",
					gomock.Any())
			},
		},
		"4/missing-warp-warns": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\n", hostFromURL(serverURL), ip4)
			},
			available: true, expected: ip4,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible,
					"The response of %q does not contain a warp field; please report this at %s",
					gomock.Any(), pp.IssueReportingURL)
			},
		},
		"4/missing-ip": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nwarp=off\n", hostFromURL(serverURL))
			},
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"The response of %q does not contain an ip field", gomock.Any())
			},
		},
		"4/unparseable-ip": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=not-an-ip\nwarp=off\n", hostFromURL(serverURL))
			},
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"Failed to parse the IP address in the response of %q (%q)",
					gomock.Any(), "not-an-ip")
			},
		},
		"4/cloudflare-ipv4-range": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=104.16.0.1\nwarp=off\n", hostFromURL(serverURL))
			},
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"The detected IP address %s is inside Cloudflare's own IP range and is not your real public IP",
					"104.16.0.1")
			},
		},
		"6/cloudflare-ipv6-range": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=2606:4700::1\nwarp=off\n", hostFromURL(serverURL))
			},
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"The detected IP address %s is inside Cloudflare's own IP range and is not your real public IP",
					"2606:4700::1")
			},
		},
		"4/not-handled": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			unmappedIP:   ipnet.IP4, // provider will have IP6 entry only
			makeResponse: func(_ string) string { return "" },
			available:    false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", "IPv4")
			},
		},
		"4/ip6-response-family-mismatch": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=off\n", hostFromURL(serverURL), ip6)
			},
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s %s", ip6.String(), "is not a valid IPv4 address")
			},
		},
		"6/ip4-response-family-mismatch": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=off\n", hostFromURL(serverURL), ip4)
			},
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Detected IP address %s %s", ip4.String(), "is not a valid IPv6 address")
			},
		},
		"4/extra-fields-ignored": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("fl=abc123\nh=%s\nip=%s\nts=1234567890\nwarp=off\ncolo=SJC\n", hostFromURL(serverURL), ip4)
			},
			available: true, expected: ip4,
		},
		"4/empty-response": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(_ string) string { return "" },
			available:    false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible,
					"The response of %q does not contain an h (host) field; please report this at %s",
					gomock.Any(), pp.IssueReportingURL)
				m.EXPECT().Noticef(pp.EmojiImpossible,
					"The response of %q does not contain a warp field; please report this at %s",
					gomock.Any(), pp.IssueReportingURL)
				m.EXPECT().Noticef(pp.EmojiError,
					"The response of %q does not contain an ip field", gomock.Any())
			},
		},
		"4/lines-without-equals": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("some-garbage\nh=%s\nip=%s\nwarp=off\nanother-line\n", hostFromURL(serverURL), ip4)
			},
			available: true, expected: ip4,
		},
		"6/warp-on": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=on\n", hostFromURL(serverURL), ip6)
			},
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError,
					"The response of %q has warp=on; the detected IP is a Cloudflare WARP egress IP, not your real public IP",
					gomock.Any())
			},
		},
		"4/warp-plus-passes": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=plus\n", hostFromURL(serverURL), ip4)
			},
			available: true, expected: ip4,
		},
		"6/missing-h-warns": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(_ string) string {
				return fmt.Sprintf("ip=%s\nwarp=off\n", ip6)
			},
			available: true, expected: ip6,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible,
					"The response of %q does not contain an h (host) field; please report this at %s",
					gomock.Any(), pp.IssueReportingURL)
			},
		},
		"6/mismatched-h": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(_ string) string {
				return fmt.Sprintf("h=wrong.example.com\nip=%s\nwarp=off\n", ip6)
			},
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible,
					"The h field %q in the response of %q does not match the expected host %q; please report this at %s",
					"wrong.example.com", gomock.Any(), gomock.Any(), pp.IssueReportingURL)
			},
		},
		"6/not-handled": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			unmappedIP:   ipnet.IP6,
			makeResponse: func(_ string) string { return "" },
			available:    false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", "IPv6")
			},
		},
		"4/request-fail": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, noServer: true, forceURL: "",
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", "", gomock.Any())
			},
		},
		"6/request-fail": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, noServer: true, forceURL: "",
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", "", gomock.Any())
			},
		},
		"4/illegal-url-escape": { //nolint:exhaustruct // test fixture sets only exercised fields
			// A URL with an illegal percent-escape fails at request preparation
			// (http.NewRequest calls url.Parse internally), so the explicit
			// url.Parse guard later in GetRawData is never reached.
			ipFamily: ipnet.IP4, noServer: true, forceURL: "http://example.com/path%zz",
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v",
					"http://example.com/path%zz", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			var provider protocol.CloudflareTrace
			var serverURL string

			if tc.noServer {
				provider = protocol.CloudflareTrace{
					ProviderName: "test",
					URL:          map[ipnet.Family]string{tc.ipFamily: tc.forceURL},
				}
			} else {
				var server *httptest.Server
				server = newSplitServer(tc.serverFamily, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					fmt.Fprint(w, tc.makeResponse(server.URL))
				}))
				t.Cleanup(server.Close)
				serverURL = server.URL

				if tc.unmappedIP == tc.ipFamily {
					// Map to opposite family so this family is unhandled.
					other := ipnet.IP6
					if tc.ipFamily == ipnet.IP6 {
						other = ipnet.IP4
					}
					provider = protocol.CloudflareTrace{
						ProviderName: "test",
						URL:          map[ipnet.Family]string{other: server.URL},
					}
				} else {
					provider = protocol.CloudflareTrace{
						ProviderName: "test",
						URL:          map[ipnet.Family]string{tc.ipFamily: server.URL},
					}
				}
			}

			if tc.prepareMockPP != nil {
				tc.prepareMockPP(serverURL, mockPP)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			prefixLen := testDefaultPrefixLen(tc.ipFamily)
			rawData := provider.GetRawData(ctx, mockPP, tc.ipFamily, prefixLen)
			require.Equal(t, tc.available, rawData.Available)
			if tc.expected.IsValid() {
				require.Equal(t, []ipnet.RawEntry{ipnet.RawEntryFrom(tc.expected, prefixLen)}, rawData.RawEntries)
			} else {
				require.Empty(t, rawData.RawEntries)
			}
		})
	}
}
