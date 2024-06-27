package monitor_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/message"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
)

func TestDescribeAll(t *testing.T) {
	t.Parallel()

	ms := make([]monitor.Monitor, 0, 5)

	mockCtrl := gomock.NewController(t)

	for range 5 {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().Describe(gomock.Any())
		ms = append(ms, m)
	}

	callback := func(_service, _params string) { /* the callback content is not relevant here. */ }
	monitor.DescribeAll(callback, ms)
}

func TestSuccessAll(t *testing.T) {
	t.Parallel()

	ms := make([]monitor.Monitor, 0, 5)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "aloha"

	for range 5 {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().Success(context.Background(), mockPP, message)
		ms = append(ms, m)
	}

	monitor.SuccessAll(context.Background(), mockPP, ms, message)
}

func TestStartAll(t *testing.T) {
	t.Parallel()

	ms := make([]monitor.Monitor, 0, 5)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "你好"

	for range 5 {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().Start(context.Background(), mockPP, message)
		ms = append(ms, m)
	}

	monitor.StartAll(context.Background(), mockPP, ms, message)
}

func TestFailureAll(t *testing.T) {
	t.Parallel()

	ms := make([]monitor.Monitor, 0, 5)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "secret"

	for range 5 {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().Failure(context.Background(), mockPP, message)
		ms = append(ms, m)
	}

	monitor.FailureAll(context.Background(), mockPP, ms, message)
}

func TestLogAll(t *testing.T) {
	t.Parallel()

	ms := make([]monitor.Monitor, 0, 5)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "forest"

	for range 5 {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().Log(context.Background(), mockPP, message)
		ms = append(ms, m)
	}

	monitor.LogAll(context.Background(), mockPP, ms, message)
}

func TestExitStatusAll(t *testing.T) {
	t.Parallel()

	ms := make([]monitor.Monitor, 0, 5)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "bye!"

	for range 5 {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().ExitStatus(context.Background(), mockPP, 42, message)
		ms = append(ms, m)
	}

	monitor.ExitStatusAll(context.Background(), mockPP, ms, 42, message)
}

func TestSendMessageAll(t *testing.T) {
	t.Parallel()

	notifierMessages := []string{"ocean", "moon"}

	for name1, tc1 := range map[string]struct {
		monitorMessages []string
	}{
		"nil":   {nil},
		"empty": {[]string{}},
		"one":   {[]string{"hi"}},
		"two":   {[]string{"hi", "hey"}},
	} {
		monitorMessage := strings.Join(tc1.monitorMessages, "\n")

		for name2, tc2 := range map[string]struct {
			ok   bool
			ping bool
		}{
			"success": {true, true},
			"log":     {true, false},
			"fail1":   {false, true},
			"fail2":   {false, false},
		} {
			t.Run(fmt.Sprintf("%s/%s", name1, name2), func(t *testing.T) {
				t.Parallel()

				ms := make([]monitor.Monitor, 0, 5)
				mockCtrl := gomock.NewController(t)
				mockPP := mocks.NewMockPP(mockCtrl)

				for range 5 {
					m := mocks.NewMockMonitor(mockCtrl)
					switch {
					case tc2.ok && tc2.ping:
						m.EXPECT().Success(context.Background(), mockPP, monitorMessage)
					case tc2.ok && len(monitorMessage) > 0:
						m.EXPECT().Log(context.Background(), mockPP, monitorMessage)
					case tc2.ok:
					default: // !tc.ok
						m.EXPECT().Failure(context.Background(), mockPP, monitorMessage)
					}
					ms = append(ms, m)
				}

				msg := message.Message{
					Ok:               tc2.ok,
					MonitorMessages:  tc1.monitorMessages,
					NotifierMessages: notifierMessages,
				}
				monitor.SendMessageAll(context.Background(), mockPP, ms, msg, tc2.ping)
			})
		}
	}
}
