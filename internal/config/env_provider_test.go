//nolint:testpackage // These tests need direct access to unexported provider readers; same-package tests are the intended shape for helper parsing logic.
package config

// vim: nowrap

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/testenv"
)

const retiredCloudflareTraceURL = "https://user:secret@example.invalid/cdn-cgi/trace?token=do-not-print#private"

//nolint:paralleltest // paralleltest should not be used because environment vars are global
func TestReadProvider(t *testing.T) {
	key := keyPrefix + "PROVIDER"
	keyDeprecated := keyPrefix + "DEPRECATED"

	var (
		none             provider.Provider
		doh              = provider.NewCloudflareDOH()
		trace            = provider.NewCloudflareTrace()
		local            = provider.NewLocal()
		localLoopback    = provider.MustNewLocalWithInterface("lo")
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
		"cloudflare.trace:retired URL": {
			ipnet.IP4, true,
			"   cloudflare.trace:" + retiredCloudflareTraceURL + " ",
			false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(
						pp.EmojiUserError,
						`%s=cloudflare.trace:... is no longer supported; use %s=cloudflare.trace`,
						key, key,
					),
					m.EXPECT().Request(pp.MessageRetiredCustomCloudflareTraceProvider),
				)
			},
		},
		"cloudflare.trace:": {
			ipnet.IP4, true, "   cloudflare.trace: ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(
						pp.EmojiUserError,
						`%s=cloudflare.trace:... is no longer supported; use %s=cloudflare.trace`,
						key, key,
					),
					m.EXPECT().Request(pp.MessageRetiredCustomCloudflareTraceProvider),
				)
			},
		},
		"cloudflare.doh": {ipnet.IP4, true, "    \tcloudflare.doh   ", false, "", none, doh, true, nil},
		"none":           {ipnet.IP4, true, "   none   ", false, "", trace, none, true, nil},
		"local":          {ipnet.IP4, true, "   local   ", false, "", trace, local, true, nil},
		"local.iface:lo": {
			ipnet.IP4, true, "   local.iface   :  lo ", false, "", trace, localLoopback, true,
			func(m *mocks.MockPP) {
				m.EXPECT().InfoOncef(pp.MessageExperimentalLocalWithInterface, pp.EmojiExperimental, `You are using the experimental "local.iface:..." provider available since version 1.15.0`)
			},
		},
		"local.iface:": {
			ipnet.IP4, true, "   local.iface: ", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().InfoOncef(pp.MessageExperimentalLocalWithInterface, pp.EmojiExperimental, `You are using the experimental "local.iface:..." provider available since version 1.15.0`),
					m.EXPECT().Noticef(pp.EmojiUserError, `%s=local.iface: must be followed by a network interface name`, key),
				)
			},
		},
		"custom":      {ipnet.IP4, true, "   url:https://url.io   ", false, "", trace, custom, true, nil},
		"custom via4": {ipnet.IP4, true, "   url.via4:https://url.io   ", false, "", trace, customVia4, true, nil},
		"custom via6": {ipnet.IP4, true, "   url.via6:https://url.io   ", false, "", trace, customVia6, true, nil},
		"custom invalid": {
			ipnet.IP4, true, "url:relative/path.txt", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s=%s does not contain a valid URL", key, "url:(redacted)")
			},
		},
		"custom via4 invalid": {
			ipnet.IP4, true, "url.via4:relative/path.txt", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s=%s does not contain a valid URL", key, "url.via4:(redacted)")
			},
		},
		"custom via6 invalid": {
			ipnet.IP4, true, "url.via6:relative/path.txt", false, "", trace, trace, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "%s=%s does not contain a valid URL", key, "url.via6:(redacted)")
			},
		},
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
		"static:trailing-comma": {ipnet.IP4, true, "static:1.1.1.1,", false, "", trace, static, true, nil},
		"static:extra-trailing-commas": {
			ipnet.IP4, true, "static:1.1.1.1,,,,,,", false, "", trace, trace, false,
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
		"static:comma-only": {
			ipnet.IP4, true, "static:,", false, "", trace, trace, false,
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
			ok := readProvider(mockPP, key, keyDeprecated, tc.ipFamily, defaultPrefixLen, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestRetiredCloudflareTrace(t *testing.T) {
	retiredProvider := "cloudflare.trace:" + retiredCloudflareTraceURL

	for name, tc := range map[string]struct {
		verbosity       pp.Verbosity
		ip4Provider     string
		ip6Provider     string
		wantOK          bool
		wantRetiredKeys []string
		wantOtherParse  string
	}{
		"ordinary cloudflare.trace": {
			verbosity:       pp.Verbose,
			ip4Provider:     "cloudflare.trace",
			ip6Provider:     "cloudflare.trace",
			wantOK:          true,
			wantRetiredKeys: nil,
			wantOtherParse:  "",
		},
		"IP4 retired syntax in normal output": {
			verbosity:       pp.Verbose,
			ip4Provider:     retiredProvider,
			ip6Provider:     "",
			wantOK:          false,
			wantRetiredKeys: []string{"IP4_PROVIDER"},
			wantOtherParse:  "Using default IP6_PROVIDER=cloudflare.doh",
		},
		"IP6 retired syntax in normal output": {
			verbosity:       pp.Verbose,
			ip4Provider:     "",
			ip6Provider:     retiredProvider,
			wantOK:          false,
			wantRetiredKeys: []string{"IP6_PROVIDER"},
			wantOtherParse:  "Using default IP4_PROVIDER=local",
		},
		"blank custom trace syntax": {
			verbosity:       pp.Verbose,
			ip4Provider:     "cloudflare.trace:",
			ip6Provider:     "cloudflare.trace",
			wantOK:          false,
			wantRetiredKeys: []string{"IP4_PROVIDER"},
			wantOtherParse:  "",
		},
		"one family retired in quiet output": {
			verbosity:       pp.Quiet,
			ip4Provider:     "cloudflare.trace",
			ip6Provider:     retiredProvider,
			wantOK:          false,
			wantRetiredKeys: []string{"IP6_PROVIDER"},
			wantOtherParse:  "",
		},
		"both families retired in normal output": {
			verbosity:       pp.Verbose,
			ip4Provider:     retiredProvider,
			ip6Provider:     retiredProvider,
			wantOK:          false,
			wantRetiredKeys: []string{"IP4_PROVIDER", "IP6_PROVIDER"},
			wantOtherParse:  "",
		},
		"both families retired in quiet output": {
			verbosity:       pp.Quiet,
			ip4Provider:     retiredProvider,
			ip6Provider:     retiredProvider,
			wantOK:          false,
			wantRetiredKeys: []string{"IP4_PROVIDER", "IP6_PROVIDER"},
			wantOtherParse:  "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			store(t, "IP4_PROVIDER", tc.ip4Provider)
			store(t, "IP6_PROVIDER", tc.ip6Provider)

			oldProviders := map[ipnet.Family]provider.Provider{
				ipnet.IP4: provider.NewLocal(),
				ipnet.IP6: provider.NewCloudflareDOH(),
			}
			providers := map[ipnet.Family]provider.Provider{
				ipnet.IP4: oldProviders[ipnet.IP4],
				ipnet.IP6: oldProviders[ipnet.IP6],
			}
			var output strings.Builder
			ok := readProviderMap(
				pp.New(&output, false, tc.verbosity),
				map[ipnet.Family]int{ipnet.IP4: 32, ipnet.IP6: 64},
				&providers,
			)
			rendered := output.String()

			require.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				require.Equal(t, "cloudflare.trace", provider.Name(providers[ipnet.IP4]))
				require.Equal(t, "cloudflare.trace", provider.Name(providers[ipnet.IP6]))
				require.NotContains(t, rendered, pp.IssueReportingURL)
				return
			}

			// Mutation caught: a failed provider read must not publish either
			// family's temporary result into the caller's provider map.
			require.Equal(t, oldProviders, providers)
			require.Contains(t, rendered, tc.wantOtherParse)

			guidanceOffset := strings.Index(rendered, pp.IssueReportingURL)
			require.NotEqual(t, -1, guidanceOffset)
			require.Equal(t, 1, strings.Count(rendered, pp.IssueReportingURL))
			for _, key := range tc.wantRetiredKeys {
				require.Contains(t, rendered, key+"=cloudflare.trace:...")
				require.Contains(t, rendered, "use "+key+"=cloudflare.trace")
				require.Less(t, strings.Index(rendered, key), guidanceOffset)
			}
			require.Contains(t, rendered, "no longer supported")

			for _, secret := range []string{"user", "secret", "token", "private"} {
				require.NotContains(t, rendered, secret)
			}
		})
	}
}

func TestRetiredCloudflareTraceStartupTranscripts(t *testing.T) {
	retiredProvider := "cloudflare.trace:" + retiredCloudflareTraceURL

	for name, tc := range map[string]struct {
		verbosity   pp.Verbosity
		ip4Provider string
		ip6Provider string
		want        string
	}{
		"one family normal": {
			verbosity:   pp.Verbose,
			ip4Provider: retiredProvider,
			ip6Provider: "cloudflare.trace",
			want: `🌟 Cloudflare DDNS
📖 Reading settings . . .
   🔸 Using default IP4_DEFAULT_PREFIX_LEN=32
   🔸 Using default IP6_DEFAULT_PREFIX_LEN=64
   😡 IP4_PROVIDER=cloudflare.trace:... is no longer supported; use IP4_PROVIDER=cloudflare.trace
   💡 If you still need a custom Cloudflare trace endpoint, open an issue at https://github.com/favonia/cloudflare-ddns/issues/new/choose
👋 Bye!
`,
		},
		"one family quiet": {
			verbosity:   pp.Quiet,
			ip4Provider: retiredProvider,
			ip6Provider: "cloudflare.trace",
			want: `😡 IP4_PROVIDER=cloudflare.trace:... is no longer supported; use IP4_PROVIDER=cloudflare.trace
💡 If you still need a custom Cloudflare trace endpoint, open an issue at https://github.com/favonia/cloudflare-ddns/issues/new/choose
`,
		},
		"both families normal": {
			verbosity:   pp.Verbose,
			ip4Provider: retiredProvider,
			ip6Provider: retiredProvider,
			want: `🌟 Cloudflare DDNS
📖 Reading settings . . .
   🔸 Using default IP4_DEFAULT_PREFIX_LEN=32
   🔸 Using default IP6_DEFAULT_PREFIX_LEN=64
   😡 IP4_PROVIDER=cloudflare.trace:... is no longer supported; use IP4_PROVIDER=cloudflare.trace
   😡 IP6_PROVIDER=cloudflare.trace:... is no longer supported; use IP6_PROVIDER=cloudflare.trace
   💡 If you still need a custom Cloudflare trace endpoint, open an issue at https://github.com/favonia/cloudflare-ddns/issues/new/choose
👋 Bye!
`,
		},
		"both families quiet": {
			verbosity:   pp.Quiet,
			ip4Provider: retiredProvider,
			ip6Provider: retiredProvider,
			want: `😡 IP4_PROVIDER=cloudflare.trace:... is no longer supported; use IP4_PROVIDER=cloudflare.trace
😡 IP6_PROVIDER=cloudflare.trace:... is no longer supported; use IP6_PROVIDER=cloudflare.trace
💡 If you still need a custom Cloudflare trace endpoint, open an issue at https://github.com/favonia/cloudflare-ddns/issues/new/choose
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			testenv.ClearAll(t)
			t.Setenv("CLOUDFLARE_API_TOKEN", "deadbeef")
			t.Setenv("IP4_PROVIDER", tc.ip4Provider)
			t.Setenv("IP6_PROVIDER", tc.ip6Provider)

			var output strings.Builder
			ppfmt := pp.New(&output, true, tc.verbosity)
			ppfmt.Infof(pp.EmojiStar, "Cloudflare DDNS")
			raw := DefaultRaw()
			require.False(t, raw.ReadEnv(ppfmt))
			ppfmt.Infof(pp.EmojiBye, "Bye!")

			rendered := output.String()
			require.Equal(t, tc.want, rendered)
			for _, secret := range []string{"user", "secret", "token", "private"} {
				require.NotContains(t, rendered, secret)
			}
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
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", "IP4_PROVIDER", "flare"),
					m.EXPECT().Infof(pp.EmojiBullet, "Using default %s=%s", "IP6_PROVIDER", "local"),
				)
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
			mockPP.EXPECT().DrainRequests(pp.MessageRetiredCustomCloudflareTraceProvider).Return(uint(0))
			ok := readProviderMap(mockPP, map[ipnet.Family]int{ipnet.IP4: 32, ipnet.IP6: 64}, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, field)
		})
	}
}
