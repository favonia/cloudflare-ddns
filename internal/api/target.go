package api

import (
	"context"
	"strings"
)

type Target interface {
	zoneID(ctx context.Context, handle *Handle) (string, error)
	zoneName(ctx context.Context, handle *Handle) (string, error)
	domain(ctx context.Context, handle *Handle) (string, error)
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

type SubdomainTarget struct {
	ZoneID    string
	Subdomain string
}

func (t *SubdomainTarget) zoneID(ctx context.Context, handle *Handle) (string, error) {
	return t.ZoneID, nil
}

func (t *SubdomainTarget) zoneName(ctx context.Context, handle *Handle) (string, error) {
	return handle.zoneName(ctx, t.ZoneID)
}

func (t *SubdomainTarget) domain(ctx context.Context, handle *Handle) (string, error) {
	zoneName, err := handle.zoneName(ctx, t.ZoneID)
	if err != nil {
		return "", err
	}

	if t.Subdomain == "" {
		return zoneName, nil
	} else {
		return strings.Join([]string{t.Subdomain, zoneName}, "."), nil
	}
}
