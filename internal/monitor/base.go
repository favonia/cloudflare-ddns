// Package monitor implements dead man's switches.
package monitor

import (
	"context"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/response"
)

//go:generate mockgen -typed -destination=../mocks/mock_monitor.go -package=mocks . Monitor

// maxReadLength is the maximum number of bytes read from an HTTP response.
const maxReadLength int64 = 102400

// Monitor is a dead man's switch, meaning that the user will be notified when the updater fails to
// detect and update the public IP address. No notifications for IP changes.
type Monitor interface {
	// Describe a monitor in a human-readable format by calling callback with service names and params.
	Describe(callback func(service, params string))

	// Success pings the monitor to prevent notifications.
	Success(ctx context.Context, ppfmt pp.PP, message string) bool

	// Start pings the monitor with the start signal.
	Start(ctx context.Context, ppfmt pp.PP, message string) bool

	// Failure immediately signals the monitor to notify the user.
	Failure(ctx context.Context, ppfmt pp.PP, message string) bool

	// Log provides additional inforamion without changing the state.
	Log(ctx context.Context, ppfmt pp.PP, message string) bool

	// ExitStatus records the exit status (as an integer in the POSIX style).
	ExitStatus(ctx context.Context, ppfmt pp.PP, code int, message string) bool
}

func SendResponse(ctx context.Context, ppfmt pp.PP, m Monitor, r response.Response) bool {
	msg := strings.Join(r.MonitorMessages, "\n")
	if r.Ok {
		return m.Log(ctx, ppfmt, msg)
	} else {
		return m.Failure(ctx, ppfmt, msg)
	}
}
