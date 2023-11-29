package updater

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

type Responses map[setter.ResponseCode][]string

func (s Responses) Register(code setter.ResponseCode, d domain.Domain) {
	s[code] = append(s[code], d.Describe())
}

func (s Responses) MonitorMessage() string {
	switch {
	case len(s[setter.ResponseUpdatesFailed]) > 0 && len(s[setter.ResponseUpdatesApplied]) > 0:
		return fmt.Sprintf("Failed to set: %s; set: %s",
			strings.Join(s[setter.ResponseUpdatesFailed], ", "),
			strings.Join(s[setter.ResponseUpdatesApplied], ", "),
		)

	case len(s[setter.ResponseUpdatesFailed]) > 0:
		return fmt.Sprintf("Failed to set: %s", strings.Join(s[setter.ResponseUpdatesFailed], ", "))

	case len(s[setter.ResponseUpdatesApplied]) > 0:
		return fmt.Sprintf("Set: %s", strings.Join(s[setter.ResponseUpdatesFailed], ", "))

	default:
		return ""
	}
}

func EnglishJoin(items []string) string {
	switch l := len(items); l {
	case 0:
		return "(none)"
	case 1:
		return items[0]
	default:
		return fmt.Sprintf("%s and %s", strings.Join(items[:l-1], ", "), items[l-1])
	}
}

func (s Responses) NotifierMessage(ipNet ipnet.Type, ip netip.Addr) string {
	switch {
	case !ip.IsValid():
		return fmt.Sprintf("Failed to detect the %s address.", ipNet.Describe())

	case len(s[setter.ResponseUpdatesFailed]) > 0 && len(s[setter.ResponseUpdatesApplied]) > 0:
		return fmt.Sprintf(
			"Possibly failed to update %s records of %s to %s; records of %s were updated.",
			ipNet.RecordType(),
			EnglishJoin(s[setter.ResponseUpdatesFailed]),
			ip.String(),
			EnglishJoin(s[setter.ResponseUpdatesApplied]),
		)

	case len(s[setter.ResponseUpdatesFailed]) > 0:
		return fmt.Sprintf(
			"Possibly failed to update %s records of %s to %s.",
			ipNet.RecordType(),
			EnglishJoin(s[setter.ResponseUpdatesFailed]),
			ip.String(),
		)

	case len(s[setter.ResponseUpdatesApplied]) > 0:
		return fmt.Sprintf(
			"Update %s records of %s to %s.",
			ipNet.RecordType(),
			EnglishJoin(s[setter.ResponseUpdatesApplied]),
			ip.String(),
		)

	default:
		return ""
	}
}
