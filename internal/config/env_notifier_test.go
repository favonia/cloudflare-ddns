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
				m.EXPECT().InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint, "You are using the experimental shoutrrr support added in version 1.12.0")
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
				m.EXPECT().InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint, "You are using the experimental shoutrrr support added in version 1.12.0")
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
		"invalid": {
			shoutrrr: "meow-meow-meow://cute",
			ok:       false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint, "You are using the experimental shoutrrr support added in version 1.12.0")
				m.EXPECT().Noticef(pp.EmojiUserError, `Could not create shoutrrr client: %v`, gomock.Any())
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
