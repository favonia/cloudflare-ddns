package monitor_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
)

func TestMonitorsDescribe(t *testing.T) {
	t.Parallel()

	var ms []monitor.Monitor

	mockCtrl := gomock.NewController(t)

	for i := 0; i < 5; i++ {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().Describe(gomock.Any())
		ms = append(ms, m)
	}

	monitor.Monitors(ms).Describe(func(service, params string) {})
}

func TestMonitorsSuccess(t *testing.T) {
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

	monitor.Monitors(ms).Success(context.Background(), mockPP, message)
}

func TestMonitorsStart(t *testing.T) {
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

	monitor.Monitors(ms).Start(context.Background(), mockPP, message)
}

func TestMonitorsFailure(t *testing.T) {
	t.Parallel()

	var ms []monitor.Monitor

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "secret"

	for i := 0; i < 5; i++ {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().Failure(context.Background(), mockPP, message)
		ms = append(ms, m)
	}

	monitor.Monitors(ms).Failure(context.Background(), mockPP, message)
}

func TestMonitorsLog(t *testing.T) {
	t.Parallel()

	var ms []monitor.Monitor

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "forest"

	for i := 0; i < 5; i++ {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().Log(context.Background(), mockPP, message)
		ms = append(ms, m)
	}

	monitor.Monitors(ms).Log(context.Background(), mockPP, message)
}

func TestMonitorsExitStatus(t *testing.T) {
	t.Parallel()

	var ms []monitor.Monitor

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "bye!"

	for i := 0; i < 5; i++ {
		m := mocks.NewMockMonitor(mockCtrl)
		m.EXPECT().ExitStatus(context.Background(), mockPP, 42, message)
		ms = append(ms, m)
	}

	monitor.Monitors(ms).ExitStatus(context.Background(), mockPP, 42, message)
}
