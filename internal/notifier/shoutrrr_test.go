package notifier_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
)

func TestShoutrrrDescripbe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := notifier.NewShoutrrr(mockPP, []string{"generic://localhost/"})
	require.True(t, ok)
	m.Describe(func(service, params string) {
		require.Equal(t, "generic", service)
	})
}
