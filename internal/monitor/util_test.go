package monitor_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
)

func TestSuccessAll(t *testing.T) {
	t.Parallel()

	var ms []monitor.Monitor

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "aloha"

	for i := 0; i < 5; i++ {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().Success(context.Background(), mockPP, message)
		ms = append(ms, m)
	}

	monitor.SuccessAll(context.Background(), mockPP, ms, message)
}

func TestStartAll(t *testing.T) {
	t.Parallel()

	var ms []monitor.Monitor

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "你好"

	for i := 0; i < 5; i++ {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().Start(context.Background(), mockPP, message)
		ms = append(ms, m)
	}

	monitor.StartAll(context.Background(), mockPP, ms, message)
}

func TestFailureAll(t *testing.T) {
	t.Parallel()

	var ms []monitor.Monitor

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "hello!"

	for i := 0; i < 5; i++ {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().Failure(context.Background(), mockPP, message)
		ms = append(ms, m)
	}

	monitor.FailureAll(context.Background(), mockPP, ms, message)
}

func TestExitStatus(t *testing.T) {
	t.Parallel()

	var ms []monitor.Monitor

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "hello!"

	for i := 0; i < 5; i++ {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().ExitStatus(context.Background(), mockPP, 42, message)
		ms = append(ms, m)
	}

	monitor.ExitStatusAll(context.Background(), mockPP, ms, 42, message)
}
