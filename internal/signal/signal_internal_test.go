package signal

import (
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestWaitForSignalsUntilDeadline(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		// A bubble-owned channel isolates the timer from operating-system signals.
		handle := Handle{channel: make(chan os.Signal)}
		start := time.Now()
		result := make(chan bool, 1)
		go func() {
			result <- handle.WaitForSignalsUntil(pp.NewSilent(), start.Add(100*time.Millisecond))
		}()

		// Catch either returning before the deadline or missing the timer expiration.
		synctest.Sleep(100*time.Millisecond - time.Nanosecond)
		select {
		case <-result:
			t.Fatal("signal wait returned before its deadline")
		default:
		}
		synctest.Sleep(time.Nanosecond)
		select {
		case interrupted := <-result:
			require.False(t, interrupted)
		default:
			t.Fatal("signal wait did not finish at its deadline")
		}
	})
}
