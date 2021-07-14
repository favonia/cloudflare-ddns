package api

import (
	"context"
)

type Target interface {
	zoneID(ctx context.Context, handle *Handle) (string, error)
	zoneName(ctx context.Context, handle *Handle) (string, error)
	domain(ctx context.Context, handle *Handle) (string, error)
	String() string
}

type FQDNTarget struct {
	Domain string
}

func (t *FQDNTarget) zoneID(ctx context.Context, handle *Handle) (string, error) {
	return handle.zoneID(ctx, t.Domain)
}

func (t *FQDNTarget) zoneName(ctx context.Context, handle *Handle) (string, error) {
	zoneID, err := handle.zoneID(ctx, t.Domain)
	if err != nil {
		return "", err
	}
	return handle.zoneName(ctx, zoneID)
}

func (t *FQDNTarget) domain(ctx context.Context, handle *Handle) (string, error) {
	return t.Domain, nil
}

func (t *FQDNTarget) String() string {
	return t.Domain
}
