package monitor

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/message"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// DescribeAll calls [Monitor.Describe] for each monitor in the group with the callback.
func DescribeAll(callback func(service, params string), ms []Monitor) {
	for _, m := range ms {
		m.Describe(callback)
	}
}

// SuccessAll calls [Monitor.Success] for each monitor in the group.
func SuccessAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Success(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

// StartAll calls [Monitor.Start] for each monitor in ms.
func StartAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Start(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

// FailureAll calls [Monitor.Failure] for each monitor in ms.
func FailureAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Failure(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

// LogAll calls [Monitor.Log] for each monitor in ms.
func LogAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Log(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

// ExitStatusAll calls [Monitor.ExitStatus] for each monitor in ms.
func ExitStatusAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, code int, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.ExitStatus(ctx, ppfmt, code, message) {
			ok = false
		}
	}
	return ok
}

// PingMessageAll calls [SendMessage] for each monitor in ms.
func PingMessageAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, msg message.MonitorMessage) bool {
	ok := true
	for _, m := range ms {
		if !PingMessage(ctx, ppfmt, m, msg) {
			ok = false
		}
	}
	return ok
}

// LogMessageAll calls [SendMessage] for each monitor in ms.
func LogMessageAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, msg message.MonitorMessage) bool {
	ok := true
	for _, m := range ms {
		if !LogMessage(ctx, ppfmt, m, msg) {
			ok = false
		}
	}
	return ok
}
