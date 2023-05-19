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

//nolint:paralleltest,funlen // paralleltest should not be used because environment vars are global
func TestReadProvider(t *testing.T) {
	key := keyPrefix + "PROVIDER"
	keyDeprecated := keyPrefix + "DEPRECATED"

	var (
		none  provider.Provider
		doh   = provider.NewCloudflareDOH
		trace = provider.NewCloudflareTrace
		local = provider.NewLocal
		ipify = provider.NewIpify()
	)

	for name, tc := range map[string]struct {
		useAlternativeIPs bool
		set               bool
		val               string
		setDeprecated     bool
		valDeprecated     string
		oldField          provider.Provider
		newField          provider.Provider
		ok                bool
		prepareMockPP     func(*mocks.MockPP)
	}{
		"nil": {
			true,
			false, "", false, "", none, none, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "none")
			},
		},
		"deprecated/empty": {
			false,
			false, "", true, "", local(true), local(true), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "local")
			},
		},
		"deprecated/cloudflare": {
			true,
			false, "", true, "    cloudflare\t   ", none, trace(true), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`%s=cloudflare is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`,
					keyDeprecated, key, key,
				)
			},
		},
		"deprecated/cloudflare.trace": {
			false,
			false, "", true, " cloudflare.trace", none, trace(false), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`%s is deprecated; use %s=%s`,
					keyDeprecated,
					key,
					"cloudflare.trace",
				)
			},
		},
		"deprecated/cloudflare.doh": {
			true,
			false, "", true, "    \tcloudflare.doh   ", none, doh(true), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`%s is deprecated; use %s=%s`,
					keyDeprecated,
					key,
					"cloudflare.doh",
				)
			},
		},
		"deprecated/unmanaged": {
			false,
			false, "", true, "   unmanaged   ", trace(false), none, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`%s is deprecated; use %s=none`,
					keyDeprecated,
					key,
				)
			},
		},
		"deprecated/local": {
			true,
			false, "", true, "   local   ", trace(false), local(true), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`%s is deprecated; use %s=%s`,
					keyDeprecated,
					key,
					"local",
				)
			},
		},
		"deprecated/ipify": {
			false,
			false, "", true, "     ipify  ", trace(false), ipify, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`%s=ipify is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`,
					keyDeprecated,
					key,
					key,
				)
			},
		},
		"deprecated/others": {
			true,
			false, "", true, "   something-else ", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a valid provider", keyDeprecated, "something-else")
			},
		},
		"conflicts": {
			false,
			true, "cloudflare.doh", true, "cloudflare.doh", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(
					pp.EmojiUserError,
					`Cannot have both %s and %s set`,
					key, keyDeprecated,
				)
			},
		},
		"empty": {
			true,
			false, "", false, "", local(false), local(false), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "local")
			},
		},
		"cloudflare": {
			false,
			true, "    cloudflare\t   ", false, "", none, none, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(
					pp.EmojiUserError,
					`%s=cloudflare is invalid; use %s=cloudflare.trace or %s=cloudflare.doh`,
					key, key, key,
				)
			},
		},
		"cloudflare.trace": {true, true, " cloudflare.trace", false, "", none, trace(true), true, nil},
		"cloudflare.doh":   {false, true, "    \tcloudflare.doh   ", false, "", none, doh(false), true, nil},
		"none":             {true, true, "   none   ", false, "", trace(true), none, true, nil},
		"local":            {false, true, "   local   ", false, "", trace(true), local(false), true, nil},
		"ipify": {
			true,
			true, "     ipify  ", false, "", trace(false), ipify, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(
					pp.EmojiUserWarning,
					`%s=ipify is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`,
					key,
					key,
					key,
				)
			},
		},
		"others": {
			false,
			true, "   something-else ", false, "", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a valid provider", key, "something-else")
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			set(t, keyDeprecated, tc.setDeprecated, tc.valDeprecated)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadProvider(mockPP, tc.useAlternativeIPs, key, keyDeprecated, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:funlen,paralleltest // environment vars are global
func TestReadProviderMap(t *testing.T) {
	var (
		none  provider.Provider
		trace = provider.NewCloudflareTrace
		doh   = provider.NewCloudflareDOH
		local = provider.NewLocal
	)

	for name, tc := range map[string]struct {
		useAlternativeIPs bool
		ip4Provider       string
		ip6Provider       string
		expected          map[ipnet.Type]provider.Provider
		ok                bool
		prepareMockPP     func(*mocks.MockPP)
	}{
		"full/true": {
			true,
			"cloudflare.trace", "local",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: trace(true),
				ipnet.IP6: local(true),
			},
			true,
			nil,
		},
		"full/false": {
			false,
			"cloudflare.trace", "local",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: trace(false),
				ipnet.IP6: local(false),
			},
			true,
			nil,
		},
		"4": {
			true,
			"local", "  ",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: local(true),
				ipnet.IP6: local(true),
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP6_PROVIDER", "local")
			},
		},
		"6": {
			false,
			"    ", "cloudflare.doh",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: doh(false),
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", "IP4_PROVIDER", "none")
			},
		},
		"empty": {
			true,
			" ", "   ",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: local(true),
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
			false,
			" flare", "   ",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: local(false),
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

			field := map[ipnet.Type]provider.Provider{ipnet.IP4: none, ipnet.IP6: local(tc.useAlternativeIPs)}
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadProviderMap(mockPP, tc.useAlternativeIPs, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, field)
		})
	}
}
