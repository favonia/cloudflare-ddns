package notifier_test

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
)

func TestDescribeAll(t *testing.T) {
	t.Parallel()

	ms := make([]notifier.Notifier, 0, 5)

	mockCtrl := gomock.NewController(t)

	for range 5 {
		m := mocks.NewMockNotifier(mockCtrl)
		m.EXPECT().Describe(gomock.Any())
		ms = append(ms, m)
	}

	callback := func(_service, _params string) { /* the callback content is not relevant here. */ }
	notifier.DescribeAll(callback, ms)
}

func TestSendAll(t *testing.T) {
	t.Parallel()

	ms := make([]notifier.Notifier, 0, 5)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "aloha"

	for range 5 {
		m := mocks.NewMockNotifier(mockCtrl)
		m.EXPECT().Send(context.Background(), mockPP, message)
		ms = append(ms, m)
	}

	notifier.SendAll(context.Background(), mockPP, message, ms)
}
