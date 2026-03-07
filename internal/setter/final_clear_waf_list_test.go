package setter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
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
		prepareMocks prepareSetterMocks
	}{
		{
			name: "already-cleaned/response-noop",
			resp: setter.ResponseNoop,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().FinalCleanWAFList(ctx, p, wafList, listDescription).Return(api.WAFListCleanupNoop)
			},
		},
		{
			name: "list-exists/delete-list/response-updated",
			resp: setter.ResponseUpdated,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().FinalCleanWAFList(ctx, p, wafList, listDescription).Return(api.WAFListCleanupUpdated)
			},
		},
		{
			name: "list-exists/clear-list-async/response-updating",
			resp: setter.ResponseUpdating,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().FinalCleanWAFList(ctx, p, wafList, listDescription).Return(api.WAFListCleanupUpdating)
			},
		},
		{
			name: "list-exists/delete-and-clear/response-failed",
			resp: setter.ResponseFailed,
			prepareMocks: func(ctx context.Context, _ func(), p *mocks.MockPP, m *mocks.MockHandle) {
				m.EXPECT().FinalCleanWAFList(ctx, p, wafList, listDescription).Return(api.WAFListCleanupFailed)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, h := newSetterHarness(t)
			h.prepare(ctx, tc.prepareMocks)

			resp := h.setter.FinalClearWAFList(ctx, h.mockPP, wafList, listDescription)
			require.Equal(t, tc.resp, resp)
		})
	}
}
