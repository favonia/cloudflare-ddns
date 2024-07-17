package config_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest,funlen // environment vars are global
func TestCheckIgnoredLinuxID(t *testing.T) {
	key := keyPrefix + "ID"
	for name, tc := range map[string]struct {
		set           bool
		val           string
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil": {
			false, "", nil,
		},
		"empty": {
			true, "", nil,
		},
		"0": {
			true, "0   ",
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserError, "%s=%s is ignored; use Docker's built-in mechanism to set %s ID", key, "0", "kitty"), //nolint:lll
					m.EXPECT().Warningf(pp.EmojiHint, "See https://github.com/favonia/cloudflare-ddns for the new Docker template"),             //nolint:lll
				)
			},
		},
		"1": {
			true, "   1   ",
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserError, "%s=%s is ignored; use Docker's built-in mechanism to set %s ID", key, "1", "kitty"), //nolint:lll
					m.EXPECT().Warningf(pp.EmojiHint, "See https://github.com/favonia/cloudflare-ddns for the new Docker template"),             //nolint:lll
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			config.CheckIgnoredLinuxID(mockPP, key, "kitty")
		})
	}
}
