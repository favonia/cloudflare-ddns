package setter_test

import (
	"maps"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
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

func liftTestPrefix(ip netip.Addr, bitLen int) netip.Prefix {
	return netip.PrefixFrom(ip, bitLen)
}

func detected(ip4, ip6 netip.Addr) map[ipnet.Family]setter.WAFTargets {
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

func detectedSets(ip4, ip6 []netip.Addr) map[ipnet.Family]setter.WAFTargets {
	result := map[ipnet.Family]setter.WAFTargets{}
	if ip4 != nil {
		prefixes := make([]netip.Prefix, 0, len(ip4))
		for _, ip := range ip4 {
			prefixes = append(prefixes, liftTestPrefix(ip, 32))
		}
		result[ipnet.IP4] = setter.NewAvailableWAFTargets(prefixes)
	}
	if ip6 != nil {
		prefixes := make([]netip.Prefix, 0, len(ip6))
		for _, ip := range ip6 {
			prefixes = append(prefixes, liftTestPrefix(ip, 64))
		}
		result[ipnet.IP6] = setter.NewAvailableWAFTargets(prefixes)
	}
	return result
}

func unavailableDetected(ip4, ip6 bool) map[ipnet.Family]setter.WAFTargets {
	result := map[ipnet.Family]setter.WAFTargets{}
	if ip4 {
		result[ipnet.IP4] = setter.NewUnavailableWAFTargets()
	}
	if ip6 {
		result[ipnet.IP6] = setter.NewUnavailableWAFTargets()
	}
	return result
}

func mergeDetected(targetSets ...map[ipnet.Family]setter.WAFTargets) map[ipnet.Family]setter.WAFTargets {
	result := map[ipnet.Family]setter.WAFTargets{}
	for _, detectedByFamily := range targetSets {
		maps.Copy(result, detectedByFamily)
	}
	return result
}
