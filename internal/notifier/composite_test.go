package notifier_test

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/response"
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

	notifier.SendAll(context.Background(), mockPP, ms, message)
}

func TestSendResponseAll(t *testing.T) {
	t.Parallel()

	monitorMessages := []string{"forest", "grass"}
	notifierMessages := []string{"ocean", "moon"}
	notifierMessage := strings.Join(notifierMessages, " ")

	for name, tc := range map[string]struct {
		ok bool
	}{
		"ok":    {true},
		"notok": {false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ns := make([]notifier.Notifier, 0, 5)
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			for range 5 {
				n := mocks.NewMockNotifier(mockCtrl)
				n.EXPECT().Send(context.Background(), mockPP, notifierMessage)
				ns = append(ns, n)
			}

			resp := response.Response{
				Ok:               tc.ok,
				MonitorMessages:  monitorMessages,
				NotifierMessages: notifierMessages,
			}
			notifier.SendResponseAll(context.Background(), mockPP, ns, resp)
		})
	}
}
