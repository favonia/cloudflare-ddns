package pp_test

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestQueued(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		run          func(pp.QueuedPP)
		prepareMocks func(ppfmt *mocks.MockPP, inner *mocks.MockPP)
	}{
		"flushed": {
			func(queued pp.QueuedPP) {
				var ppfmt pp.PP = queued
				if ppfmt.IsShowing(pp.Info) {
					ppfmt.Infof(pp.EmojiBullet, "Test")
					ppfmt = ppfmt.Indent()
				}
				ppfmt.Noticef(pp.EmojiNotify, "some message")
				ppfmt.Suppress(pp.MessageDetectionTimeouts)
				ppfmt.BlankLineIfVerbose()
				ppfmt.NoticeOncef(pp.MessageIP4DetectionFails, pp.EmojiHint, "cannot do IPv4")

				queued.Flush()
			},
			func(ppfmt, inner *mocks.MockPP) {
				ppfmt.EXPECT().Indent().Return(inner)
				ppfmt.EXPECT().IsShowing(gomock.Any()).Return(true)
				gomock.InOrder(
					ppfmt.EXPECT().Infof(pp.EmojiBullet, "Test"),
					inner.EXPECT().Noticef(pp.EmojiNotify, "some message"),
					inner.EXPECT().Suppress(pp.MessageDetectionTimeouts),
					inner.EXPECT().BlankLineIfVerbose(),
					inner.EXPECT().NoticeOncef(pp.MessageIP4DetectionFails, pp.EmojiHint, "cannot do IPv4"),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			mockPPInner := mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP, mockPPInner)
			}

			tc.run(pp.NewQueued(mockPP))
		})
	}
}
