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

	var ms []notifier.Notifier

	mockCtrl := gomock.NewController(t)

	for i := 0; i < 5; i++ {
		m := mocks.NewMockNotifier(mockCtrl)
		m.EXPECT().Describe(gomock.Any())
		ms = append(ms, m)
	}

	callback := func(_service, _params string) { /* the callback content is not relevant here. */ }
	notifier.DescribeAll(callback, ms)
}

func TestSendAll(t *testing.T) {
	t.Parallel()

	var ms []notifier.Notifier

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "aloha"

	for i := 0; i < 5; i++ {
		m := mocks.NewMockNotifier(mockCtrl)
		m.EXPECT().Send(context.Background(), mockPP, message)
		ms = append(ms, m)
	}

	notifier.SendAll(context.Background(), mockPP, message, ms)
}
