package setter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestFinalClearWAFList(t *testing.T) {
	t.Parallel()

	const listName = "list"
	const listDescription = "My List"
	wafList := api.WAFList{AccountID: "account", Name: listName}

	cases := []struct {
		name         string
		resp         setter.ResponseCode
		prepareMocks func(ctx context.Context, cancel func(), p *mocks.MockPP, m *mocks.MockHandle)
	}{
		{
			name: "deleted",
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().FinalClearWAFListAsync(ctx, p, wafList, listDescription).Return(true, true),
					p.EXPECT().Noticef(pp.EmojiDeletion, "The list %s was deleted", wafList.Describe()),
				)
			},
		},
		{
			name: "cleared",
			resp: setter.ResponseUpdating,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				gomock.InOrder(
					m.EXPECT().FinalClearWAFListAsync(ctx, p, wafList, listDescription).Return(false, true),
					p.EXPECT().Noticef(pp.EmojiClear, "The list %s is being cleared (asynchronously)", wafList.Describe()),
				)
			},
		},
		{
			name: "delete-fail/clear-fail",
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().FinalClearWAFListAsync(ctx, p, wafList, listDescription).Return(false, false)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newSetterHarness(t)
			if tc.prepareMocks != nil {
				tc.prepareMocks(h.ctx, h.cancel, h.mockPP, h.mockHandle)
			}

			resp := h.setter.FinalClearWAFList(h.ctx, h.mockPP, wafList, listDescription)
			require.Equal(t, tc.resp, resp)
		})
	}
}
