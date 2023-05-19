package config_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

//nolint:funlen,paralleltest // environment vars are global
func TestReadProviderMap(t *testing.T) {
	var (
		none            provider.Provider
		cloudflareTrace = provider.NewCloudflareTrace()
		cloudflareDOH   = provider.NewCloudflareDOH()
		local           = provider.NewLocal()
	)

	for name, tc := range map[string]struct {
		ip4Provider   string
		ip6Provider   string
		expected      map[ipnet.Type]provider.Provider
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"full": {
			"cloudflare.trace", "local",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: cloudflareTrace,
				ipnet.IP6: local,
			},
			true,
			nil,
		},
		"4": {
			"local", "  ",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: local,
				ipnet.IP6: local,
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP6_PROVIDER", "local")
			},
		},
		"6": {
			"    ", "cloudflare.doh",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: cloudflareDOH,
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP4_PROVIDER", "none")
			},
		},
		"empty": {
			" ", "   ",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: local,
			},
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP4_PROVIDER", "none"),
					m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP6_PROVIDER", "local"),
				)
			},
		},
		"illformed": {
			" flare", "   ",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: local,
			},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a valid provider", "IP4_PROVIDER", "flare")
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			store(t, "IP4_PROVIDER", tc.ip4Provider)
			store(t, "IP6_PROVIDER", tc.ip6Provider)

			field := map[ipnet.Type]provider.Provider{ipnet.IP4: none, ipnet.IP6: local}
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadProviderMap(mockPP, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, field)
		})
	}
}
