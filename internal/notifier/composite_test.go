package notifier_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
)

func TestComposedDescribe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)

	ns1 := make([]notifier.Notifier, 0, 3)
	for range 3 {
		n := mocks.NewMockNotifier(mockCtrl)
		n.EXPECT().Describe(gomock.Any()).DoAndReturn(
			func(yield func(string, string) bool) {
				yield("name", "params")
			},
		)
		ns1 = append(ns1, n)
	}
	ns2 := make([]notifier.Notifier, 0, 2)
	for range 2 {
		n := mocks.NewMockNotifier(mockCtrl)
		ns2 = append(ns2, n)
	}
	c := notifier.NewComposed(notifier.NewComposed(ns1...), notifier.NewComposed(ns2...))

	count := 0
	for range c.Describe {
		count++
		if count >= 3 {
			break
		}
	}
	require.Equal(t, 3, count)
}

func TestComposedSend(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ss []string
	}{
		"nil":   {nil},
		"empty": {[]string{}},
		"one":   {[]string{"hi"}},
		"two":   {[]string{"hi", "hey"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ns := make([]notifier.Notifier, 0, 5)
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			msg := notifier.Message(tc.ss)

			for range 5 {
				n := mocks.NewMockNotifier(mockCtrl)
				n.EXPECT().Send(context.Background(), mockPP, msg).Return(true)
				ns = append(ns, n)
			}

			ok := notifier.NewComposed(ns...).Send(context.Background(), mockPP, msg)
			require.True(t, ok)
		})
	}
}
