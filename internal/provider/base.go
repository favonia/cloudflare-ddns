package provider

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate mockgen -destination=../mocks/mock_provider.go -package=mocks . Provider

type Provider interface {
	Name() string
	GetIP(context.Context, pp.PP, ipnet.Type) netip.Addr
}

func Name(p Provider) string {
	if p == nil {
		return "none"
	}

	return p.Name()
}
