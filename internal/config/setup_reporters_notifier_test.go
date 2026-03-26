package config_test

// vim: nowrap

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest // environment vars are global
func TestSetupReportersNotifier(t *testing.T) {
	for name, tc := range map[string]struct {
		shoutrrr      string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
		check         func(*testing.T, heartbeat.Heartbeat, notifier.Notifier)
	}{
		"empty": {
			shoutrrr:      "",
			ok:            true,
			prepareMockPP: nil,
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"single": {
			shoutrrr: "generic+https://example.com/api/v1/postStuff",
			ok:       true,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint, "You are using the experimental shoutrrr support available since version 1.12.0")
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				ns, ok := nt.(notifier.Composed)
				require.True(t, ok)
				require.Len(t, ns, 1)
				s, ok := ns[0].(notifier.Shoutrrr)
				require.True(t, ok)
				require.Equal(t, []string{"Generic"}, s.ServiceDescriptions)
			},
		},
		"multiple": {
			shoutrrr: "generic+https://example.com/api/v1/postStuff\npushover://shoutrrr:token@userKey",
			ok:       true,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint, "You are using the experimental shoutrrr support available since version 1.12.0")
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				ns, ok := nt.(notifier.Composed)
				require.True(t, ok)
				require.Len(t, ns, 1)
				s, ok := ns[0].(notifier.Shoutrrr)
				require.True(t, ok)
				require.Equal(t, []string{"Generic", "Pushover"}, s.ServiceDescriptions)
			},
		},
		"multiple folded by compose": {
			shoutrrr: "generic+https://example.com/api/v1/postStuff pushover://shoutrrr:token@userKey",
			ok:       false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"The %s non-empty line of SHOUTRRR contains spaces, which suggests that multiple URLs were folded onto one line",
					"1st")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you meant multiple URLs, put each URL on its own line; if this is one URL, percent-encode spaces")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you are using YAML folded block style >, use literal block style | instead")
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"mixed newline and folded": {
			shoutrrr: "generic+https://example.com/api/v1/postStuff\npushover://shoutrrr:token@userKey ifttt://hey/?events=1",
			ok:       false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"The %s non-empty line of SHOUTRRR contains spaces, which suggests that multiple URLs were folded onto one line",
					"2nd")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you meant multiple URLs, put each URL on its own line; if this is one URL, percent-encode spaces")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you are using YAML folded block style >, use literal block style | instead")
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"single URL with raw space": {
			shoutrrr: "generic+https://example.com/hook?title=hello world",
			ok:       true,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserWarning,
					"The %s non-empty line of SHOUTRRR contains spaces",
					"1st")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"Percent-encode spaces to suppress this warning")
				m.EXPECT().InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint, "You are using the experimental shoutrrr support available since version 1.12.0")
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				ns, ok := nt.(notifier.Composed)
				require.True(t, ok)
				require.Len(t, ns, 1)
				s, ok := ns[0].(notifier.Shoutrrr)
				require.True(t, ok)
				require.Equal(t, []string{"Generic"}, s.ServiceDescriptions)
			},
		},
		"one URL-like token plus trailing junk": {
			shoutrrr: "pushover://shoutrrr:token@userKey not-a-url",
			ok:       false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"The %s non-empty line of SHOUTRRR contains spaces, which suggests that multiple URLs were folded onto one line",
					"1st")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you meant multiple URLs, put each URL on its own line; if this is one URL, percent-encode spaces")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you are using YAML folded block style >, use literal block style | instead")
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"first token not URL-like": {
			shoutrrr: "not-a-url generic+https://example.com/api/v1/postStuff",
			ok:       false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"The %s non-empty line of SHOUTRRR contains spaces, which suggests that multiple URLs were folded onto one line",
					"1st")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you meant multiple URLs, put each URL on its own line; if this is one URL, percent-encode spaces")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you are using YAML folded block style >, use literal block style | instead")
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"invalid": {
			shoutrrr: "meow-meow-meow://cute",
			ok:       false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint, "You are using the experimental shoutrrr support available since version 1.12.0")
				m.EXPECT().Noticef(pp.EmojiUserError, `Failed to create a Shoutrrr client: %v`, gomock.Any())
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"warn line plus fail line": {
			shoutrrr: "generic+https://example.com/hook?title=hello world\npushover://shoutrrr:token@userKey not-a-url",
			ok:       false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"The %s non-empty line of SHOUTRRR contains spaces, which suggests that multiple URLs were folded onto one line",
					"2nd")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you meant multiple URLs, put each URL on its own line; if this is one URL, percent-encode spaces")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you are using YAML folded block style >, use literal block style | instead")
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"blank lines are skipped but non-empty line numbers stay original": {
			shoutrrr: "\n  generic+https://example.com/hook?title=hello world  \n\n   \npushover://shoutrrr:token@userKey ifttt://hey/?events=1\n",
			ok:       false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"The %s non-empty line of SHOUTRRR contains spaces, which suggests that multiple URLs were folded onto one line",
					"5th")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you meant multiple URLs, put each URL on its own line; if this is one URL, percent-encode spaces")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you are using YAML folded block style >, use literal block style | instead")
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"repeated spaces": {
			shoutrrr: "generic+https://example.com/api/v1/postStuff  pushover://shoutrrr:token@userKey",
			ok:       false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"The %s non-empty line of SHOUTRRR contains spaces, which suggests that multiple URLs were folded onto one line",
					"1st")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you meant multiple URLs, put each URL on its own line; if this is one URL, percent-encode spaces")
				m.EXPECT().Infof(
					pp.EmojiHint,
					"If you are using YAML folded block style >, use literal block style | instead")
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			unset(t, "HEALTHCHECKS", "UPTIMEKUMA", "SHOUTRRR")
			if tc.shoutrrr != "" {
				store(t, "SHOUTRRR", tc.shoutrrr)
			}

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			hb, nt, ok := config.SetupReporters(mockPP)
			require.Equal(t, tc.ok, ok)
			tc.check(t, hb, nt)
		})
	}
}
