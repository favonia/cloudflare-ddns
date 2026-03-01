package setter_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestNewAcceptsNilManagedRecordRegex(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockHandle := mocks.NewMockHandle(mockCtrl)

	s, ok := setter.New(mockPP, mockHandle, nil)
	require.True(t, ok)
	require.NotNil(t, s)
}

func TestNewAcceptsCompiledManagedRecordRegex(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockHandle := mocks.NewMockHandle(mockCtrl)

	s, ok := setter.New(mockPP, mockHandle, regexp.MustCompile(""))
	require.True(t, ok)
	require.NotNil(t, s)
}
