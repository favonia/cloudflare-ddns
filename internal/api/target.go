package api

import (
	"context"
)

type Target interface {
	zoneID(ctx context.Context, handle *Handle) (string, bool)
	zoneName(ctx context.Context, handle *Handle) (string, bool)
	domain(ctx context.Context, handle *Handle) (string, bool)
	String() string
}

type FQDNTarget struct {
	Domain string
}

func (t *FQDNTarget) zoneID(ctx context.Context, handle *Handle) (string, bool) {
	return handle.zoneID(ctx, t.Domain)
}

func (t *FQDNTarget) zoneName(ctx context.Context, handle *Handle) (string, bool) {
	zoneID, ok := handle.zoneID(ctx, t.Domain)
	if !ok {
		return "", false
	}
	return handle.zoneName(ctx, zoneID)
}

func (t *FQDNTarget) domain(ctx context.Context, handle *Handle) (string, bool) {
	return t.Domain, true
}

func (t *FQDNTarget) String() string {
	return t.Domain
}
