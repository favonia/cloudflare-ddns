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
		mockPP.EXPECT().Warningf(pp.EmojiUserWarning, "You are running this tool as root, which is usually a bad idea")
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
		calls = append(calls, mockPP.EXPECT().Warningf(pp.EmojiUserWarning, "You are running this tool as root, which is usually a bad idea"))
	}
	calls = append(calls,
		mockPP.EXPECT().Warningf(pp.EmojiUserError, "PUID=%s is ignored; use Docker's built-in mechanism to set user ID", "1000"),
		mockPP.EXPECT().Warningf(pp.EmojiUserError, "PGID=%s is ignored; use Docker's built-in mechanism to set group ID", "1000"),
		mockPP.EXPECT().Warningf(pp.EmojiHint, "See https://github.com/favonia/cloudflare-ddns for the new Docker template"),
	)
	gomock.InOrder(calls...)

	config.CheckRoot(mockPP)
}
