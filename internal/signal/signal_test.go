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

//nolint:paralleltest // Signals are process-global.
func TestWaitForSignalsUntil(t *testing.T) {
	for name, sig := range map[string]syscall.Signal{
		"sigint":  syscall.SIGINT,
		"sigterm": syscall.SIGTERM,
	} {
		t.Run(name, func(t *testing.T) {
			mockPP := mocks.NewMockPP(gomock.NewController(t))
			mockPP.EXPECT().Noticef(pp.EmojiSignal, "Caught signal: %v", sig)

			handle := signal.Setup()
			// Setup has registered the buffered signal channel before delivery.
			require.NoError(t, syscall.Kill(os.Getpid(), sig))
			require.True(t, handle.WaitForSignalsUntil(mockPP, time.Now().Add(time.Second)))
		})
	}
}

//nolint:paralleltest // Signals are process-global.
func TestNotifyContext(t *testing.T) {
	for name, sig := range map[string]syscall.Signal{
		"explicit cancellation": 0,
		"sigint":                syscall.SIGINT,
		"sigterm":               syscall.SIGTERM,
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := signal.NotifyContext(context.Background())
			t.Cleanup(cancel)
			if sig == 0 {
				cancel()
			} else {
				// NotifyContext has registered the signal before delivery.
				require.NoError(t, syscall.Kill(os.Getpid(), sig))
			}

			select {
			case <-ctx.Done():
				require.ErrorIs(t, ctx.Err(), context.Canceled)
			case <-time.After(time.Second):
				t.Fatal("notification context was not canceled")
			}
		})
	}
}
