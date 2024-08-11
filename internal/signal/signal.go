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

// Signals contains the signals to mask and catch.
//
//nolint:gochecknoglobals
var Signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}

// Setup masks signals in [Signals] and return the handle.
func Setup() Handle {
	chanSignal := make(chan os.Signal, len(Signals))
	signal.Notify(chanSignal, Signals...)

	return Handle{channel: chanSignal}
}

// NotifyContext gives a copy of the context that will be canceled by signals in [Signals].
func NotifyContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, Signals...)
}

// TearDown undoes what Setup does. This is only for testing.
func (h Handle) TearDown() {
	signal.Stop(h.channel)
}

// ReportSignalsUntil waits for a period of time. It returns true if it is interrupted by signals in [Signals].
func (h Handle) ReportSignalsUntil(ppfmt pp.PP, t time.Time) bool {
	timer := time.NewTimer(time.Until(t))
	for {
		select {
		case sig := <-h.channel:
			if !timer.Stop() {
				<-timer.C
			}
			ppfmt.Noticef(pp.EmojiSignal, "Caught signal: %v", sig)
			return true
		case <-timer.C:
			return false
		}
	}
}
