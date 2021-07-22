package api

import (
	"context"
)

type Target interface {
	zone(ctx context.Context, handle *Handle) (string, bool)
	domain(ctx context.Context, handle *Handle) (string, bool)
	String() string
}

type FQDNTarget struct {
	Domain string
}

func (t *FQDNTarget) zone(ctx context.Context, handle *Handle) (string, bool) {
	return handle.zoneOfDomain(ctx, t.Domain)
}

func (t *FQDNTarget) domain(_ context.Context, _ *Handle) (string, bool) {
	return t.Domain, true
}

func (t *FQDNTarget) String() string {
	return t.Domain
}
