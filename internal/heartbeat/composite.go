package heartbeat

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Composed represents the composite of multiple heartbeat services.
type Composed []BasicHeartbeat

var _ Heartbeat = Composed{}

// NewComposed creates a new composed heartbeat service.
func NewComposed(services ...BasicHeartbeat) Composed {
	heartbeats := make([]BasicHeartbeat, 0, len(services))
	for _, hb := range services {
		if hb == nil {
			continue
		}
		if list, composed := hb.(Composed); composed {
			heartbeats = append(heartbeats, list...)
		} else {
			heartbeats = append(heartbeats, hb)
		}
	}
	return Composed(heartbeats)
}

// Describe calls [Heartbeat.Describe] for each service in the group.
func (hs Composed) Describe(yield func(name string, params string) bool) {
	for _, hb := range hs {
		for name, params := range hb.Describe {
			if !yield(name, params) {
				return
			}
		}
	}
}

// Ping calls [Heartbeat.Ping] for each service in the group.
func (hs Composed) Ping(ctx context.Context, ppfmt pp.PP, message Message) bool {
	ok := true
	for _, hb := range hs {
		ok = ok && hb.Ping(ctx, ppfmt, message)
	}
	return ok
}

// Start calls [Heartbeat.Start] for each service in the group.
func (hs Composed) Start(ctx context.Context, ppfmt pp.PP, message string) bool {
	ok := true
	for _, hb := range hs {
		if extended, okType := hb.(Heartbeat); okType {
			ok = ok && extended.Start(ctx, ppfmt, message)
		}
	}
	return ok
}

// Exit calls [Heartbeat.Exit] for each service in the group.
func (hs Composed) Exit(ctx context.Context, ppfmt pp.PP, message string) bool {
	ok := true
	for _, hb := range hs {
		if extended, okType := hb.(Heartbeat); okType {
			ok = ok && extended.Exit(ctx, ppfmt, message)
		}
	}
	return ok
}

// Log calls [Heartbeat.Log] for each service in the group.
func (hs Composed) Log(ctx context.Context, ppfmt pp.PP, msg Message) bool {
	ok := true
	for _, hb := range hs {
		if extended, okType := hb.(Heartbeat); okType {
			ok = ok && extended.Log(ctx, ppfmt, msg)
		} else if !msg.OK {
			ok = ok && hb.Ping(ctx, ppfmt, msg)
		}
	}
	return ok
}
