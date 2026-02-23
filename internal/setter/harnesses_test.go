package setter_test

import (
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

func dnsRecord(id api.ID, ip netip.Addr, params api.RecordParams) api.Record {
	return api.Record{
		ID:           id,
		IP:           ip,
		RecordParams: params,
	}
}

func wafItem(cidr string, id api.ID) api.WAFListItem {
	return api.WAFListItem{
		ID:     id,
		Prefix: netip.MustParsePrefix(cidr),
	}
}

func detected(ip4, ip6 netip.Addr) map[ipnet.Type][]netip.Addr {
	var ip4s []netip.Addr
	var ip6s []netip.Addr
	if ip4.IsValid() {
		ip4s = []netip.Addr{ip4}
	}
	if ip6.IsValid() {
		ip6s = []netip.Addr{ip6}
	}
	return map[ipnet.Type][]netip.Addr{
		ipnet.IP4: ip4s,
		ipnet.IP6: ip6s,
	}
}

func detectedSets(ip4, ip6 []netip.Addr) map[ipnet.Type][]netip.Addr {
	return map[ipnet.Type][]netip.Addr{
		ipnet.IP4: ip4,
		ipnet.IP6: ip6,
	}
}
