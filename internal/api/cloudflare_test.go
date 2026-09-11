package api_test

import (
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestNewValid(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockPP := mocks.NewMockPP(mockCtrl)

		_, _, ok := newHandle(t, mockPP)
		require.True(t, ok)
	})
}

func TestNewEmptyToken(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, auth, _ := newServerAuth(t)

	auth.Token = ""
	mockPP.EXPECT().Noticef(pp.EmojiUserError, "Failed to prepare the Cloudflare API client: %v", gomock.Any())
	h, ok := auth.New(mockPP, defaultHandleOptions())
	require.False(t, ok)
	require.Nil(t, h)
}
