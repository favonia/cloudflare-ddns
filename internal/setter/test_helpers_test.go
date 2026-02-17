package setter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

type setterHarness struct {
	ctx        context.Context
	cancel     context.CancelFunc
	mockPP     *mocks.MockPP
	mockHandle *mocks.MockHandle
	setter     setter.Setter
}

func newSetterHarness(t *testing.T) setterHarness {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockHandle := mocks.NewMockHandle(mockCtrl)

	s, ok := setter.New(mockPP, mockHandle)
	require.True(t, ok)

	return setterHarness{
		ctx:        ctx,
		cancel:     cancel,
		mockPP:     mockPP,
		mockHandle: mockHandle,
		setter:     s,
	}
}

func wrapCancelAsDelete(cancel func()) func(context.Context, pp.PP, ipnet.Type, domain.Domain, api.ID, api.DeletionMode) bool {
	return func(context.Context, pp.PP, ipnet.Type, domain.Domain, api.ID, api.DeletionMode) bool {
		cancel()
		return false
	}
}
