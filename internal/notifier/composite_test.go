package notifier_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/message"
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

	notifier.SendAll(context.Background(), mockPP, ms, message)
}

func TestSendMessageAll(t *testing.T) {
	t.Parallel()

	monitorMessages := []string{"forest", "grass"}

	for name1, tc1 := range map[string]struct {
		notifierMessages []string
	}{
		"nil":   {nil},
		"empty": {[]string{}},
		"one":   {[]string{"hi"}},
		"two":   {[]string{"hi", "hey"}},
	} {
		notifierMessage := strings.Join(tc1.notifierMessages, " ")

		for name2, tc2 := range map[string]struct {
			ok bool
		}{
			"ok":    {true},
			"notok": {false},
		} {
			t.Run(fmt.Sprintf("%s/%s", name1, name2), func(t *testing.T) {
				t.Parallel()

				ns := make([]notifier.Notifier, 0, 5)
				mockCtrl := gomock.NewController(t)
				mockPP := mocks.NewMockPP(mockCtrl)

				for range 5 {
					n := mocks.NewMockNotifier(mockCtrl)
					if len(tc1.notifierMessages) > 0 {
						n.EXPECT().Send(context.Background(), mockPP, notifierMessage)
					}
					ns = append(ns, n)
				}

				msg := message.Message{
					OK:               tc2.ok,
					MonitorMessages:  monitorMessages,
					NotifierMessages: tc1.notifierMessages,
				}
				notifier.SendMessageAll(context.Background(), mockPP, ns, msg)
			})
		}
	}
}
