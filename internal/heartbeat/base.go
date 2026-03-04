// Package heartbeat implements dead-man's-switch services.
package heartbeat

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate go tool mockgen -typed -destination=../mocks/mock_heartbeat.go -package=mocks . BasicHeartbeat,Heartbeat

// maxReadLength is the maximum number of bytes read from an HTTP response.
const maxReadLength int64 = 102400

// BasicHeartbeat is a dead man's switch, meaning that the user will be notified when the updater fails to
// detect and update the public IP address. No notifications for IP changes.
type BasicHeartbeat interface {
	// Describe a heartbeat service by its name and parameters.
	Describe(yield func(service, params string) bool)

	// Ping with OK=true prevent notifications.
	// Ping with OK=false immediately notifies the user.
	Ping(ctx context.Context, ppfmt pp.PP, msg Message) bool
}

// Heartbeat provides more advanced dead-man's-switch features.
type Heartbeat interface {
	BasicHeartbeat

	// Start pings the service with the start signal.
	Start(ctx context.Context, ppfmt pp.PP, message string) bool

	// Exit pings the service with the successful exiting signal.
	Exit(ctx context.Context, ppfmt pp.PP, message string) bool

	// Log with OK=true provides additional information without changing the state.
	// Log with OK=false immediately notifies the user.
	Log(ctx context.Context, ppfmt pp.PP, msg Message) bool
}
