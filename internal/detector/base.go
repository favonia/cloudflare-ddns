package detector

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate mockgen -destination=../mocks/mock_policy.go -package=mocks . Policy

type Policy interface {
	Name() string
	GetIP(context.Context, pp.PP, ipnet.Type) netip.Addr
}

func Name(p Policy) string {
	if p == nil {
		return "unmanaged"
	}

	return p.Name()
}
