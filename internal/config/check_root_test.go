// vim: nowrap
package config_test

import (
	"syscall"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest // environment vars are global
func TestCheckRoot(t *testing.T) {
	unset(t, "PUID", "PGID")

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	if syscall.Geteuid() == 0 {
		mockPP.EXPECT().Noticef(pp.EmojiUserWarning, "You are running this updater as root, which is usually a bad idea")
	}
	config.CheckRoot(mockPP)
}

//nolint:paralleltest // environment vars are global
func TestCheckRootWithOldConfigs(t *testing.T) {
	set(t, "PUID", true, "1000")
	set(t, "PGID", true, "1000")

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	var calls []any
	if syscall.Geteuid() == 0 {
		calls = append(calls, mockPP.EXPECT().Noticef(pp.EmojiUserWarning, "You are running this updater as root, which is usually a bad idea"))
	}
	calls = append(calls,
		mockPP.EXPECT().Noticef(pp.EmojiUserWarning, "PUID=%s is ignored since 1.13.0; use Docker's built-in mechanism to set user ID", "1000"),
		mockPP.EXPECT().Noticef(pp.EmojiUserWarning, "PGID=%s is ignored since 1.13.0; use Docker's built-in mechanism to set group ID", "1000"),
		mockPP.EXPECT().InfoOncef(pp.MessageUpdateDockerTemplate, pp.EmojiHint, "See %s for the new Docker template", pp.ManualURL),
	)
	gomock.InOrder(calls...)

	config.CheckRoot(mockPP)
}
