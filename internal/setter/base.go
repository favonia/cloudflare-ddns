package setter

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate mockgen -destination=../mocks/mock_setter.go -package=mocks . Setter

type Setter interface {
	Set(ctx context.Context, ppfmt pp.PP, Domain api.Domain, IPNetwork ipnet.Type, IP netip.Addr) bool
}
