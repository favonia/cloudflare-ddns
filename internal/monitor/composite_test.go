package monitor_test

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

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

	monitor.SuccessAll(context.Background(), mockPP, message, ms)
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

	monitor.StartAll(context.Background(), mockPP, message, ms)
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

	monitor.FailureAll(context.Background(), mockPP, message, ms)
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

	monitor.LogAll(context.Background(), mockPP, message, ms)
}

func TestMonitorsExitStatus(t *testing.T) {
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

	monitor.ExitStatusAll(context.Background(), mockPP, 42, message, ms)
}
