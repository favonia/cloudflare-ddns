// Package notifier implements push notifications.
package notifier

import (
	"context"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/response"
)

//go:generate mockgen -typed -destination=../mocks/mock_notifier.go -package=mocks . Notifier

// Notifier is an abstract service for push notifications.
type Notifier interface {
	// Describe a notifier in a human-readable format by calling callback with service names and params.
	Describe(callback func(service, params string))

	// Send out a message.
	Send(ctx context.Context, ppfmt pp.PP, msg string) bool
}

func SendResponse(ctx context.Context, ppfmt pp.PP, n Notifier, r response.Response) bool {
	if len(r.NotifierMessages) == 0 {
		return true
	}
	return n.Send(ctx, ppfmt, strings.Join(r.NotifierMessages, " "))
}
