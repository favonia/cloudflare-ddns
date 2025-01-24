// vim: nowrap
package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest // paralleltest should not be used because environment vars are global
func TestReadAndAppendShoutrrrURL(t *testing.T) {
	key := keyPrefix + "SHOUTRRR"

	type not = notifier.Notifier

	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      not
		newField      func(*testing.T, not)
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"unset": {
			false, "", nil,
			func(t *testing.T, n not) {
				t.Helper()
				require.Nil(t, n)
			},
			true,
			nil,
		},
		"empty": {
			true, "", nil,
			func(t *testing.T, n not) {
				t.Helper()
				require.Nil(t, n)
			},
			true,
			nil,
		},
		"generic": {
			true, "generic+https://example.com/api/v1/postStuff",
			nil,
			func(t *testing.T, n not) {
				t.Helper()
				ns, ok := n.(notifier.Composed)
				require.True(t, ok)
				require.Len(t, ns, 1)
				s, ok := ns[0].(notifier.Shoutrrr)
				require.True(t, ok)
				require.Equal(t, []string{"Generic"}, s.ServiceDescriptions)
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint, "You are using the experimental shoutrrr support added in version 1.12.0")
			},
		},
		"ill-formed": {
			true, "meow-meow-meow://cute",
			nil,
			func(t *testing.T, n not) {
				t.Helper()
				require.Nil(t, n)
			},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint, "You are using the experimental shoutrrr support added in version 1.12.0")
				m.EXPECT().Noticef(pp.EmojiUserError, `Could not create shoutrrr client: %v`, gomock.Any())
			},
		},
		"multiple": {
			true, "generic+https://example.com/api/v1/postStuff\npushover://shoutrrr:token@userKey",
			nil,
			func(t *testing.T, n not) {
				t.Helper()
				ns, ok := n.(notifier.Composed)
				require.True(t, ok)
				require.Len(t, ns, 1)
				s, ok := ns[0].(notifier.Shoutrrr)
				require.True(t, ok)
				require.Equal(t, []string{"Generic", "Pushover"}, s.ServiceDescriptions)
			},
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint, "You are using the experimental shoutrrr support added in version 1.12.0")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadAndAppendShoutrrrURL(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			tc.newField(t, field)
		})
	}
}
