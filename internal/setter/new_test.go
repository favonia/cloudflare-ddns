package setter_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestNewAcceptsHandle(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockHandle := mocks.NewMockHandle(mockCtrl)

	s := setter.New(mockHandle)
	require.NotNil(t, s)
}
