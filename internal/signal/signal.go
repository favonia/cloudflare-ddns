// Package signal implements the handling of signals.
package signal

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Handle encapsulates a channel for masked signals.
type Handle struct {
	channel chan os.Signal
}

// signals contains the signals to mask and catch.
//
//nolint:gochecknoglobals
var signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}

// Setup masks interrupt and termination signals and returns the handle.
func Setup() Handle {
	chanSignal := make(chan os.Signal, len(signals))
	signal.Notify(chanSignal, signals...)

	return Handle{channel: chanSignal}
}

// NotifyContext gives a copy of the context that will be canceled by interrupt or termination signals.
func NotifyContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, signals...)
}

// WaitForSignalsUntil waits for a period of time.
// It returns true if it is interrupted by an interrupt or termination signal.
func (h Handle) WaitForSignalsUntil(ppfmt pp.PP, t time.Time) bool {
	timer := time.NewTimer(time.Until(t))
	for {
		select {
		case sig := <-h.channel:
			ppfmt.Noticef(pp.EmojiSignal, "Caught signal: %v", sig)
			return true
		case <-timer.C:
			return false
		}
	}
}
