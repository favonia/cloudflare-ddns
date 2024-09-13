package notifier

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Composed represents the composite of multiple notifiers.
type Composed []Notifier

var _ Notifier = Composed{}

// NewComposed creates a new composed notifier.
func NewComposed(nots ...Notifier) Composed {
	ns := make([]Notifier, 0, len(nots))
	for _, n := range nots {
		if n == nil {
			continue
		}
		if list, composed := n.(Composed); composed {
			ns = append(ns, list...)
		} else {
			ns = append(ns, n)
		}
	}
	return Composed(ns)
}

// Describe calls [Notifier.Describe] for each notifier in the group with the callback.
func (ns Composed) Describe(yield func(name string, params string) bool) {
	for _, n := range ns {
		for name, params := range n.Describe {
			if !yield(name, params) {
				return
			}
		}
	}
}

// Send calls [Notifier.Send] for each notifier in the group.
func (ns Composed) Send(ctx context.Context, ppfmt pp.PP, msg Message) bool {
	ok := true
	for _, n := range ns {
		ok = ok && n.Send(ctx, ppfmt, msg)
	}
	return ok
}
