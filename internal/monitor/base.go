// Package monitor implements dead man's switches.
package monitor

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate mockgen -typed -destination=../mocks/mock_monitor.go -package=mocks . Monitor

// Monitor is a dead man's switch, meaning that the user will be notified when the updater fails to
// detect and update the public IP address. No notifications for IP changes.
type Monitor interface {
	// Describe a monitor in a human-readable format by calling callback with service names and params.
	Describe(callback func(service, params string))

	// Success pings the monitor to prevent notifications.
	Success(context.Context, pp.PP, string) bool

	// Start pings the monitor with the start signal.
	Start(context.Context, pp.PP, string) bool

	// Failure immediately signals the monitor to notify the user.
	Failure(context.Context, pp.PP, string) bool

	// Log provides additional inforamion without changing the state.
	Log(context.Context, pp.PP, string) bool

	// ExitStatus records the exit status (as an integer in the POSIX style).
	ExitStatus(context.Context, pp.PP, int, string) bool
}
