package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

//nolint:paralleltest // paralleltest should not be used because environment vars are global
func TestReadProvider(t *testing.T) {
	key := keyPrefix + "PROVIDER"
	keyDeprecated := keyPrefix + "DEPRECATED"

	var (
		none          provider.Provider
		doh           = provider.NewCloudflareDOH()
		trace         = provider.NewCloudflareTrace()
		local         = provider.NewLocal()
		localLoopback = provider.NewLocalWithInterface("lo")
		ipify         = provider.NewIpify()
		custom        = provider.MustNewCustomURL("https://url.io")
	)

	for name, tc := range map[string]struct {
		set           bool
		val           string
		setDeprecated bool
		valDeprecated string
		oldField      provider.Provider
		newField      provider.Provider
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil": {
			false, "", false, "", none, none, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "none")
			},
		},
		"deprecated/empty": {
			false, "", true, "", local, local, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "local")
			},
		},
		"deprecated/cloudflare": {
			false, "", true, "    cloudflare\t   ", none, trace, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserWarning,
					`%s=cloudflare is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`,
					keyDeprecated, key, key,
				)
			},
		},
		"deprecated/cloudflare.trace": {
			false, "", true, " cloudflare.trace", none, trace, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserWarning,
					`%s is deprecated; use %s=%s`,
					keyDeprecated,
					key,
					"cloudflare.trace",
				)
			},
		},
		"deprecated/cloudflare.doh": {
			false, "", true, "    \tcloudflare.doh   ", none, doh, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserWarning,
					`%s is deprecated; use %s=%s`,
					keyDeprecated,
					key,
					"cloudflare.doh",
				)
			},
		},
		"deprecated/unmanaged": {
			false, "", true, "   unmanaged   ", trace, none, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserWarning,
					`%s is deprecated; use %s=none`,
					keyDeprecated,
					key,
				)
			},
		},
		"deprecated/local": {
			false, "", true, "   local   ", trace, local, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserWarning,
					`%s is deprecated; use %s=%s`,
					keyDeprecated,
					key,
					"local",
				)
			},
		},
		"deprecated/ipify": {
			false, "", true, "     ipify  ", trace, ipify, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserWarning,
					`%s=ipify is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`,
					keyDeprecated,
					key,
					key,
				)
			},
		},
		"deprecated/others": {
			false, "", true, "   something-else ", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", keyDeprecated, "something-else")
			},
		},
		"conflicts": {
			true, "cloudflare.doh", true, "cloudflare.doh", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					`Cannot have both %s and %s set`,
					key, keyDeprecated,
				)
			},
		},
		"empty": {
			false, "", false, "", local, local, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Use default %s=%s", key, "local")
			},
		},
		"cloudflare": {
			true, "    cloudflare\t   ", false, "", none, none, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					`%s=cloudflare is invalid; use %s=cloudflare.trace or %s=cloudflare.doh`,
					key, key, key,
				)
			},
		},
		"cloudflare.trace": {true, " cloudflare.trace", false, "", none, trace, true, nil},
		"cloudflare.doh":   {true, "    \tcloudflare.doh   ", false, "", none, doh, true, nil},
		"none":             {true, "   none   ", false, "", trace, none, true, nil},
		"local":            {true, "   local   ", false, "", trace, local, true, nil},
		"local:lo":         {true, "   local   :  lo ", false, "", trace, localLoopback, true, nil},
		"local:": {
			true, "   local: ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					`%s=local: must be followed by a network interface name`,
					key,
				)
			},
		},
		"custom": {true, "   url:https://url.io   ", false, "", trace, custom, true, nil},
		"ipify": {
			true, "     ipify  ", false, "", trace, ipify, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserWarning,
					`%s=ipify is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`,
					key,
					key,
					key,
				)
			},
		},
		"others": {
			true, "   something-else ", false, "", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", key, "something-else")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			set(t, keyDeprecated, tc.setDeprecated, tc.valDeprecated)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadProvider(mockPP, key, keyDeprecated, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadProviderMap(t *testing.T) {
	var (
		none  provider.Provider
		trace = provider.NewCloudflareTrace()
		doh   = provider.NewCloudflareDOH()
		local = provider.NewLocal()
	)

	for name, tc := range map[string]struct {
		use1001       bool
		ip4Provider   string
		ip6Provider   string
		expected      map[ipnet.Type]provider.Provider
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"full/true": {
			true,
			"cloudflare.trace", "local",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: trace,
				ipnet.IP6: local,
			},
			true,
			nil,
		},
		"full/false": {
			false,
			"cloudflare.trace", "local",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: trace,
				ipnet.IP6: local,
			},
			true,
			nil,
		},
		"4": {
			true,
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
			false,
			"    ", "cloudflare.doh",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: doh,
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
			false,
			" flare", "   ",
			map[ipnet.Type]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: local,
			},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", "IP4_PROVIDER", "flare")
			},
		},
	} {
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
