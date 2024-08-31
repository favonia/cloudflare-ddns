package monitor

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type monitors []BasicMonitor

var _ Monitor = monitors{}

// NewComposed creates a new composed monitor.
func NewComposed(mons ...BasicMonitor) monitors {
	ms := make([]BasicMonitor, 0, len(mons))
	for _, m := range mons {
		if m == nil {
			continue
		}
		if list, composed := m.(monitors); composed {
			ms = append(ms, list...)
		} else {
			ms = append(ms, m)
		}
	}
	return monitors(ms)
}

// Describe calls [Monitor.Describe] for each monitor in the group with the callback.
func (ms monitors) Describe(yield func(name string, params string) bool) {
	for _, m := range ms {
		for name, params := range m.Describe {
			if !yield(name, params) {
				return
			}
		}
	}
}

// Ping calls [Monitor.Ping] for each monitor in the group.
func (ms monitors) Ping(ctx context.Context, ppfmt pp.PP, message Message) bool {
	ok := true
	for _, m := range ms {
		ok = ok && m.Ping(ctx, ppfmt, message)
	}
	return ok
}

// Start calls [Monitor.Start] for each monitor in ms.
func (ms monitors) Start(ctx context.Context, ppfmt pp.PP, message string) bool {
	ok := true
	for _, m := range ms {
		if em, extended := m.(Monitor); extended {
			ok = ok && em.Start(ctx, ppfmt, message)
		}
	}
	return ok
}

// Exit calls [Monitor.Exit] for each monitor in ms.
func (ms monitors) Exit(ctx context.Context, ppfmt pp.PP, message string) bool {
	ok := true
	for _, m := range ms {
		if em, extended := m.(Monitor); extended {
			ok = ok && em.Exit(ctx, ppfmt, message)
		}
	}
	return ok
}

// Log calls [Monitor.Log] for each monitor in the group.
func (ms monitors) Log(ctx context.Context, ppfmt pp.PP, msg Message) bool {
	ok := true
	for _, m := range ms {
		if em, extended := m.(Monitor); extended {
			ok = ok && em.Log(ctx, ppfmt, msg)
		} else if !msg.OK {
			ok = ok && m.Ping(ctx, ppfmt, msg)
		}
	}
	return ok
}
