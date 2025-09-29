package domainexp

import (
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

// DomainHostID is a domain with an (optional) host ID.
type DomainHostID struct {
	domain.Domain
	ipnet.HostID
}
