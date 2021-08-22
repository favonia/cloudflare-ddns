package detector_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/detector"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestHTTPIsManaged(t *testing.T) {
	t.Parallel()

	policy := detector.HTTP{
		PolicyName: "",
		URL:        nil,
	}

	require.True(t, policy.IsManaged())
}

func TestHTTPString(t *testing.T) {
	t.Parallel()

	policy := detector.HTTP{
		PolicyName: "very secret name",
		URL:        nil,
	}

	require.Equal(t, "very secret name", policy.String())
}

//nolint:funlen
func TestHTTPGetIP(t *testing.T) {
	ip4 := net.ParseIP("1.2.3.4").To4()
	ip6 := net.ParseIP("::1:2:3:4:5:6").To16()

	ip4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, ip4.String())
	}))
	defer ip4Server.Close()
	ip6Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, ip6.String())
	}))
	defer ip6Server.Close()
	dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not an ip")
	}))
	defer dummy.Close()

	t.Run("group", func(t *testing.T) {
		for name, tc := range map[string]struct {
			urlKey        ipnet.Type
			url           string
			ipNet         ipnet.Type
			expected      net.IP
			prepareMockPP func(*mocks.MockPP)
		}{
			"4":    {ipnet.IP4, ip4Server.URL, ipnet.IP4, ip4, nil},
			"6":    {ipnet.IP6, ip6Server.URL, ipnet.IP6, ip6, nil},
			"4to6": {ipnet.IP6, ip4Server.URL, ipnet.IP6, ip4.To16(), nil},
			"6to4": {
				ipnet.IP4, ip6Server.URL, ipnet.IP4, nil,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(
						pp.EmojiError, "%q is not a valid %s address",
						ip6,
						"IPv4",
					)
				},
			},
			"4-nil1": {ipnet.IP4, dummy.URL, ipnet.IP4, nil, nil},
			"6-nil1": {ipnet.IP6, dummy.URL, ipnet.IP6, nil, nil},
			"4-nil2": {
				ipnet.IP4, "", ipnet.IP4, nil,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(
						pp.EmojiError,
						"Failed to send HTTP(S) request to %q: %v",
						"",
						gomock.Any(),
					)
				},
			},
			"6-nil2": {
				ipnet.IP6, "", ipnet.IP6, nil,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(
						pp.EmojiError,
						"Failed to send HTTP(S) request to %q: %v",
						"",
						gomock.Any(),
					)
				},
			},
			"4-nil3": {
				ipnet.IP4, ip4Server.URL, ipnet.IP6, nil,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv6")
				},
			},
			"6-nil3": {
				ipnet.IP6, ip6Server.URL, ipnet.IP4, nil,
				func(m *mocks.MockPP) {
					m.EXPECT().Warningf(pp.EmojiImpossible, "Unhandled IP network: %s", "IPv4")
				},
			},
		} {
			tc := tc
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				mockCtrl := gomock.NewController(t)

				policy := &detector.HTTP{
					PolicyName: "",
					URL: map[ipnet.Type]string{
						tc.urlKey: tc.url,
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
	})
}
