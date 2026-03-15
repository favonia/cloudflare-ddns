package setter_test

import (
	"maps"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func dnsRecord(id api.ID, ip netip.Addr, params api.RecordParams) api.Record {
	return api.Record{
		ID:           id,
		IP:           ip,
		RecordParams: params,
	}
}

// wafItemFixture keeps WAF test fixtures explicit, including comments.
type wafItemFixture struct {
	prefix  string
	id      api.ID
	comment string
}

func wafItem(fixture wafItemFixture) api.WAFListItem {
	return api.WAFListItem{
		ID:      fixture.id,
		Prefix:  netip.MustParsePrefix(fixture.prefix),
		Comment: fixture.comment,
	}
}

func detected(ip4, ip6 netip.Addr) map[ipnet.Family]provider.Targets {
	return detectedSets(
		func() []netip.Addr {
			if ip4.IsValid() {
				return []netip.Addr{ip4}
			}
			return []netip.Addr{}
		}(),
		func() []netip.Addr {
			if ip6.IsValid() {
				return []netip.Addr{ip6}
			}
			return []netip.Addr{}
		}(),
	)
}

func detectedSets(ip4, ip6 []netip.Addr) map[ipnet.Family]provider.Targets {
	result := map[ipnet.Family]provider.Targets{}
	if ip4 != nil {
		result[ipnet.IP4] = provider.NewAvailableTargets(ip4)
	}
	if ip6 != nil {
		result[ipnet.IP6] = provider.NewAvailableTargets(ip6)
	}
	return result
}

func unavailableDetected(ip4, ip6 bool) map[ipnet.Family]provider.Targets {
	result := map[ipnet.Family]provider.Targets{}
	if ip4 {
		result[ipnet.IP4] = provider.NewUnavailableTargets()
	}
	if ip6 {
		result[ipnet.IP6] = provider.NewUnavailableTargets()
	}
	return result
}

func mergeDetected(targetSets ...map[ipnet.Family]provider.Targets) map[ipnet.Family]provider.Targets {
	result := map[ipnet.Family]provider.Targets{}
	for _, detectedByFamily := range targetSets {
		maps.Copy(result, detectedByFamily)
	}
	return result
}
