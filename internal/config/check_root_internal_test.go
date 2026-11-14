package config

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// The warning depends on the effective UID, not on the privileges of the test
// runner. Exercise the private decision directly so both cases run on every host.
func TestCheckEffectiveUID(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		uid  int
		warn bool
	}{
		{name: "root", uid: 0, warn: true},
		{name: "ordinary user", uid: 1000, warn: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mockPP := mocks.NewMockPP(gomock.NewController(t))
			if tc.warn {
				mockPP.EXPECT().Noticef(pp.EmojiUserWarning,
					"You are running this updater as root, which is usually a bad idea")
			}
			checkEffectiveUID(mockPP, tc.uid)
		})
	}
}
