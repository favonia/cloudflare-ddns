package monitor

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Monitors is the composite monitor that will notify a group of monitors simultaneously.
type Monitors []Monitor

// Describe calls [Monitor.Describe] for each monitor in the group with the callback.
func (ms Monitors) Describe(callback func(service, params string)) {
	for _, m := range ms {
		m.Describe(callback)
	}
}

// Success calls [Monitor.Success] for each monitor in the group.
func (ms Monitors) Success(ctx context.Context, ppfmt pp.PP, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Success(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

// Start calls [Monitor.Start] for each monitor in the group.
func (ms Monitors) Start(ctx context.Context, ppfmt pp.PP, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Start(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

// Failure calls [Monitor.Failure] for each monitor in the group.
func (ms Monitors) Failure(ctx context.Context, ppfmt pp.PP, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Failure(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

// Log calls [Monitor.Log] for each monitor in the group.
func (ms Monitors) Log(ctx context.Context, ppfmt pp.PP, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Log(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

// ExitStatus calls [Monitor.ExitStatus] for each monitor in the group.
func (ms Monitors) ExitStatus(ctx context.Context, ppfmt pp.PP, code int, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.ExitStatus(ctx, ppfmt, code, message) {
			ok = false
		}
	}
	return ok
}
