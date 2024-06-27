package signal_test

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/signal"
)

//nolint:paralleltest //signals are global
func TestSleep(t *testing.T) {
	for name, tc := range map[string]struct {
		alarmDelay    time.Duration
		signalDelay   time.Duration
		signal        syscall.Signal
		expected      bool
		prepareMockPP func(m *mocks.MockPP)
	}{
		"no-signal": {time.Second / 10, 0, 0, true, nil},
		"sigint": {
			time.Second, time.Second / 10, syscall.SIGINT, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiSignal, "Caught signal: %v", syscall.SIGINT)
			},
		},
		"sigterm": {
			time.Second, time.Second / 10, syscall.SIGTERM, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiSignal, "Caught signal: %v", syscall.SIGTERM)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			done := make(chan struct{}, 1)
			signalSelf := func() {
				if tc.signalDelay > 0 {
					time.Sleep(tc.signalDelay)
					err := syscall.Kill(os.Getpid(), tc.signal)
					require.NoError(t, err)
				}
				done <- struct{}{}
			}

			sig := signal.Setup()
			go signalSelf()
			target := time.Now().Add(tc.alarmDelay)
			res := sig.SleepUntil(mockPP, target)
			<-done
			sig.TearDown()

			require.Equal(t, tc.expected, res)
		})
	}
}

//nolint:paralleltest //signals are global
func TestNotifyContext(t *testing.T) {
	delta := time.Second / 10
	for name, tc := range map[string]struct {
		signalDelay time.Duration
		signal      syscall.Signal
	}{
		"no signal": {0, 0},
		"sigint":    {time.Second / 10, syscall.SIGINT},
		"sigterm":   {time.Second / 10, syscall.SIGTERM},
	} {
		t.Run(name, func(t *testing.T) {
			done := make(chan struct{}, 1)
			signalSelf := func(cancel func()) {
				if tc.signalDelay > 0 {
					time.Sleep(tc.signalDelay)
					err := syscall.Kill(os.Getpid(), tc.signal)
					require.NoError(t, err)
				} else {
					cancel()
				}
				done <- struct{}{}
			}

			ctxWithSignals, cancelCtxWithSignals := signal.NotifyContext(context.Background())
			startTime := time.Now()
			expectedEndTime := startTime.Add(tc.signalDelay)
			go signalSelf(cancelCtxWithSignals)
			<-ctxWithSignals.Done()
			<-done
			require.WithinDuration(t, expectedEndTime, time.Now(), delta)
		})
	}
}
