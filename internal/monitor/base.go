// Package monitor implements dead man's switches.
package monitor

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate go tool mockgen -typed -destination=../mocks/mock_monitor.go -package=mocks . BasicMonitor,Monitor

// maxReadLength is the maximum number of bytes read from an HTTP response.
const maxReadLength int64 = 102400

// BasicMonitor is a dead man's switch, meaning that the user will be notified when the updater fails to
// detect and update the public IP address. No notifications for IP changes.
type BasicMonitor interface {
	// Describe a monitor as a service name and its parameters.
	Describe(yield func(service, params string) bool)

	// Ping with OK=true prevent notifications.
	// Ping with OK=false immediately notifies the user.
	Ping(ctx context.Context, ppfmt pp.PP, msg Message) bool
}

// Monitor provides more advanced features.
type Monitor interface {
	BasicMonitor

	// Start pings the monitor with the start signal.
	Start(ctx context.Context, ppfmt pp.PP, message string) bool

	// Exit pings the monitor with the successful exiting signal.
	Exit(ctx context.Context, ppfmt pp.PP, message string) bool

	// Log with OK=true provides additional information without changing the state.
	// Log with OK=false immediately notifies the user.
	Log(ctx context.Context, ppfmt pp.PP, msg Message) bool
}
