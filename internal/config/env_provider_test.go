package config_test

// vim: nowrap

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
		none             provider.Provider
		doh              = provider.NewCloudflareDOH()
		trace            = provider.NewCloudflareTrace()
		traceCustom      = provider.NewCloudflareTraceCustom("https://1.1.1.1/cdn-cgi/trace")
		local            = provider.NewLocal()
		localLoopback    = provider.NewLocalWithInterface("lo")
		ipify            = provider.NewIpify()
		custom           = provider.MustNewCustomURL("https://url.io")
		customVia4       = provider.MustNewCustomURLVia4("https://url.io")
		customVia6       = provider.MustNewCustomURLVia6("https://url.io")
		static           = provider.MustNewStatic(ipnet.IP4, 32, "1.1.1.1")
		staticMulti      = provider.MustNewStatic(ipnet.IP4, 32, "2.2.2.2,1.1.1.1,2.2.2.2")
		staticEmpty      = provider.NewStaticEmpty()
		fileProvider     = provider.MustNewFile("/etc/ips.txt")
		debugUnavailable = provider.NewDebugUnavailable()
	)

	for name, tc := range map[string]struct {
		ipFamily      ipnet.Family
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
			ipnet.IP4, false, "", false, "", none, none, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%s", key, "none")
			},
		},
		"deprecated/empty": {
			ipnet.IP4, false, "", true, "", local, local, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%s", key, "local")
			},
		},
		"deprecated/cloudflare": {
			ipnet.IP4, false, "", true, "    cloudflare\t   ", none, trace, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, `%s=cloudflare is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`, keyDeprecated, key, key)
			},
		},
		"deprecated/cloudflare.trace": {
			ipnet.IP4, false, "", true, " cloudflare.trace", none, trace, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, `%s is deprecated; use %s=%s`, keyDeprecated, key, "cloudflare.trace")
			},
		},
		"deprecated/cloudflare.doh": {
			ipnet.IP4, false, "", true, "    \tcloudflare.doh   ", none, doh, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, `%s is deprecated; use %s=%s`, keyDeprecated, key, "cloudflare.doh")
			},
		},
		"deprecated/unmanaged": {
			ipnet.IP4, false, "", true, "   unmanaged   ", trace, none, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, `%s is deprecated; use %s=none`, keyDeprecated, key)
			},
		},
		"deprecated/local": {
			ipnet.IP4, false, "", true, "   local   ", trace, local, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, `%s is deprecated; use %s=%s`, keyDeprecated, key, "local")
			},
		},
		"deprecated/ipify": {
			ipnet.IP4, false, "", true, "     ipify  ", trace, ipify, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, `%s=ipify is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`, keyDeprecated, key, key)
			},
		},
		"deprecated/others": {
			ipnet.IP4, false, "", true, "   something-else ", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", keyDeprecated, "something-else")
			},
		},
		"conflicts": {
			ipnet.IP4, true, "cloudflare.doh", true, "cloudflare.doh", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `Cannot have both %s and %s set`, key, keyDeprecated)
			},
		},
		"empty": {
			ipnet.IP4, false, "", false, "", local, local, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%s", key, "local")
			},
		},
		"cloudflare": {
			ipnet.IP4, true, "    cloudflare\t   ", false, "", none, none, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `%s=cloudflare is invalid; use %s=cloudflare.trace or %s=cloudflare.doh`, key, key, key)
			},
		},
		"cloudflare.trace": {ipnet.IP4, true, " cloudflare.trace", false, "", none, trace, true, nil},
		"cloudflare.trace:https://1.1.1.1/cdn-cgi/trace": {
			ipnet.IP4, true, "   cloudflare.trace:https://1.1.1.1/cdn-cgi/trace ", false, "", trace, traceCustom, true,
			func(m *mocks.MockPP) {
				m.EXPECT().InfoOncef(pp.MessageUndocumentedCustomCloudflareTraceProvider, pp.EmojiHint, `You are using the undocumented "cloudflare.trace" provider with a custom URL; this will soon be removed`)
			},
		},
		"cloudflare.trace:": {
			ipnet.IP4, true, "   cloudflare.trace: ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().InfoOncef(pp.MessageUndocumentedCustomCloudflareTraceProvider, pp.EmojiHint, `You are using the undocumented "cloudflare.trace" provider with a custom URL; this will soon be removed`),
					m.EXPECT().Noticef(pp.EmojiUserError, `%s=cloudflare.trace: must be followed by a URL`, key),
				)
			},
		},
		"cloudflare.doh": {ipnet.IP4, true, "    \tcloudflare.doh   ", false, "", none, doh, true, nil},
		"none":           {ipnet.IP4, true, "   none   ", false, "", trace, none, true, nil},
		"local":          {ipnet.IP4, true, "   local   ", false, "", trace, local, true, nil},
		"local.iface:lo": {
			ipnet.IP4, true, "   local.iface   :  lo ", false, "", trace, localLoopback, true,
			func(m *mocks.MockPP) {
				m.EXPECT().InfoOncef(pp.MessageExperimentalLocalWithInterface, pp.EmojiHint, `You are using the experimental "local.iface" provider available since version 1.15.0`)
			},
		},
		"local.iface:": {
			ipnet.IP4, true, "   local.iface: ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `%s=local.iface: must be followed by a network interface name`, key)
			},
		},
		"custom":      {ipnet.IP4, true, "   url:https://url.io   ", false, "", trace, custom, true, nil},
		"custom via4": {ipnet.IP4, true, "   url.via4:https://url.io   ", false, "", trace, customVia4, true, nil},
		"custom via6": {ipnet.IP4, true, "   url.via6:https://url.io   ", false, "", trace, customVia6, true, nil},
		"static:1.1.1.1": {
			ipnet.IP4, true, "   static   :  1.1.1.1 ", false, "", trace, static, true,
			nil,
		},
		"static:2.2.2.2,1.1.1.1,2.2.2.2": {
			ipnet.IP4, true, "   static   :  2.2.2.2, 1.1.1.1, 2.2.2.2 ", false, "", trace, staticMulti, true,
			nil,
		},
		"static.empty": {
			ipnet.IP4, true, "   static.empty   ", false, "", trace, staticEmpty, true,
			nil,
		},
		"static:trailing-comma": {
			ipnet.IP4, true, "static:1.1.1.1,", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`The %s entry in %s is empty (check for extra commas)`, "2nd", key)
			},
		},
		"static:double-comma": {
			ipnet.IP4, true, "static:1.1.1.1,,2.2.2.2", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`The %s entry in %s is empty (check for extra commas)`, "2nd", key)
			},
		},
		"static:loopback": {
			ipnet.IP4, true, "static:127.0.0.1", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`The %s entry (%q) in %s %s`,
					"1st", "127.0.0.1", key, "is a loopback address")
			},
		},
		"static:unspecified": {
			ipnet.IP4, true, "static:0.0.0.0", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`The %s entry (%q) in %s %s`,
					"1st", "0.0.0.0", key, "is an unspecified address")
			},
		},
		"static:link-local": {
			ipnet.IP4, true, "static:169.254.1.1", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`The %s entry (%q) in %s %s`,
					"1st", "169.254.1.1", key, "is a link-local address")
			},
		},
		"static:is4in6": {
			ipnet.IP6, true, "static:::ffff:1.1.1.1", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserError,
						`The %s entry (%q) in %s %s`,
						"1st", "::ffff:1.1.1.1", key, "is an IPv4-mapped IPv6 address"),
					m.EXPECT().InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint,
						"An IPv4-mapped IPv6 address is an IPv4 address in disguise. It cannot be used for routing IPv6 traffic. If you need to use it for DNS, please open an issue at %s",
						pp.IssueReportingURL),
				)
			},
		},
		"static:1::1%eth0": {
			ipnet.IP4, true, "   static   :  1::1%eth0 ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					`Failed to parse the %s entry (%q) in %s as an IP address or an IP address in CIDR notation`,
					"1st", "1::1%eth0", key,
				)
			},
		},
		"static:family-mismatch": {
			ipnet.IP4, true, "static:2001:db8::1", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					`The %s entry (%q) in %s %s`,
					"1st", "2001:db8::1", key, "is not a valid IPv4 address",
				)
			},
		},
		"static": {
			ipnet.IP4, true, "   static: ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `%s=static: must be followed by at least one IP address`, key)
			},
		},
		"file:/etc/ips.txt": {
			ipnet.IP4, true, "   file:/etc/ips.txt ", false, "", trace, fileProvider, true,
			nil,
		},
		"file:": {
			ipnet.IP4, true, "   file: ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `%s=file: must be followed by a file path`, key)
			},
		},
		"file:relative": {
			ipnet.IP4, true, "file:relative/path.txt", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserError,
						"The path %s is not absolute; to use an absolute path, prefix it with /",
						"relative/path.txt"),
					m.EXPECT().Noticef(pp.EmojiHint,
						"Try setting %s=file:%s", key, "/relative/path.txt"),
				)
			},
		},
		"ipify": {
			ipnet.IP4, true, "     ipify  ", false, "", trace, ipify, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, `%s=ipify is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`, key, key, key)
			},
		},
		"debug.unavailable": {
			ipnet.IP4, true, "   debug.unavailable   ", false, "", trace, debugUnavailable, true,
			func(m *mocks.MockPP) {
				m.EXPECT().InfoOncef(pp.MessageUndocumentedDebugUnavailableProvider, pp.EmojiHint,
					`You are using the undocumented "debug.unavailable" provider`)
			},
		},
		"debug.unavailable:": {
			ipnet.IP4, true, "   debug.unavailable: ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", key, "debug.unavailable:")
			},
		},
		"others": {
			ipnet.IP4, true, "   something-else ", false, "", ipify, ipify, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", key, "something-else")
			},
		},
		"debug.const:1.1.1.1": {
			ipnet.IP4, true, "   debug.const   :  1.1.1.1 ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", key, "debug.const   :  1.1.1.1")
			},
		},
		"debug.const:2.2.2.2,1.1.1.1,2.2.2.2": {
			ipnet.IP4, true, "   debug.const   :  2.2.2.2, 1.1.1.1, 2.2.2.2 ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", key, "debug.const   :  2.2.2.2, 1.1.1.1, 2.2.2.2")
			},
		},
		"debug.const:1::1%eth0": {
			ipnet.IP4, true, "   debug.const   :  1::1%eth0 ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", key, "debug.const   :  1::1%eth0")
			},
		},
		"debug.const": {
			ipnet.IP4, true, "   debug.const: ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", key, "debug.const:")
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
			defaultPrefixLen := map[ipnet.Family]int{ipnet.IP4: 32, ipnet.IP6: 64}[tc.ipFamily]
			ok := config.ReadProvider(mockPP, key, keyDeprecated, tc.ipFamily, defaultPrefixLen, &field)
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
		expected      map[ipnet.Family]provider.Provider
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"full/true": {
			true,
			"cloudflare.trace", "local",
			map[ipnet.Family]provider.Provider{
				ipnet.IP4: trace,
				ipnet.IP6: local,
			},
			true,
			nil,
		},
		"full/false": {
			false,
			"cloudflare.trace", "local",
			map[ipnet.Family]provider.Provider{
				ipnet.IP4: trace,
				ipnet.IP6: local,
			},
			true,
			nil,
		},
		"ip4 via4 and ip6 via6": {
			true,
			"url.via4:https://url4.io", "url.via6:https://url6.io",
			map[ipnet.Family]provider.Provider{
				ipnet.IP4: provider.MustNewCustomURLVia4("https://url4.io"),
				ipnet.IP6: provider.MustNewCustomURLVia6("https://url6.io"),
			},
			true,
			nil,
		},
		"ip4 via6": {
			true,
			"url.via6:https://url4.io", "local",
			map[ipnet.Family]provider.Provider{
				ipnet.IP4: provider.MustNewCustomURLVia6("https://url4.io"),
				ipnet.IP6: local,
			},
			true,
			nil,
		},
		"ip6 via4": {
			true,
			"local", "url.via4:https://url6.io",
			map[ipnet.Family]provider.Provider{
				ipnet.IP4: local,
				ipnet.IP6: provider.MustNewCustomURLVia4("https://url6.io"),
			},
			true,
			nil,
		},
		"none/none": {
			true,
			"none", "none",
			map[ipnet.Family]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: none,
			},
			true,
			nil,
		},
		"4": {
			true,
			"local", "  ",
			map[ipnet.Family]provider.Provider{
				ipnet.IP4: local,
				ipnet.IP6: local,
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%s", "IP6_PROVIDER", "local")
			},
		},
		"6": {
			false,
			"    ", "cloudflare.doh",
			map[ipnet.Family]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: doh,
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%s", "IP4_PROVIDER", "none")
			},
		},
		"empty": {
			true,
			" ", "   ",
			map[ipnet.Family]provider.Provider{
				ipnet.IP4: none,
				ipnet.IP6: local,
			},
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%s", "IP4_PROVIDER", "none"),
					m.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%s", "IP6_PROVIDER", "local"),
				)
			},
		},
		"malformed": {
			false,
			" flare", "   ",
			map[ipnet.Family]provider.Provider{
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

			field := map[ipnet.Family]provider.Provider{ipnet.IP4: none, ipnet.IP6: local}
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadProviderMap(mockPP, map[ipnet.Family]int{ipnet.IP4: 32, ipnet.IP6: 64}, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, field)
		})
	}
}
