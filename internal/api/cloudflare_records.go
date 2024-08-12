package api

import (
	"context"
	"net/netip"
	"slices"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ListZones returns a list of zone IDs with the zone name.
func (h CloudflareHandle) ListZones(ctx context.Context, ppfmt pp.PP, name string) ([]string, bool) {
	// WithZoneFilters does not work with the empty zone name,
	// and the owner of the DNS root zone will not be managed by Cloudflare anyways!
	if name == "" {
		return []string{}, true
	}

	if ids := h.cache.listZones.Get(name); ids != nil {
		return ids.Value(), true
	}

	res, err := h.cf.ListZonesContext(ctx, cloudflare.WithZoneFilters(name, "", ""))
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to check the existence of a zone named %q: %v", name, err)
		return nil, false
	}

	// The operation went through. No need to perform any sanity checking in near future!
	h.skipSanityCheckToken()

	ids := make([]string, 0, len(res.Result))
	for _, zone := range res.Result {
		// The list of possible statuses was at https://api.cloudflare.com/#zone-list-zones
		// but the documentation is missing now.
		switch zone.Status {
		case "active": // fully working
			ids = append(ids, zone.ID)
		case
			"deactivated",  // violating term of service, etc.
			"initializing", // the setup was just started?
			"moved",        // domain registrar not pointing to Cloudflare
			"pending":      // the setup was not completed
			ppfmt.Warningf(pp.EmojiWarning, "Zone %q is %q; your Cloudflare setup is incomplete; some features might not work as expected", name, zone.Status) //nolint:lll
			ids = append(ids, zone.ID)
		case
			"deleted": // archived, pending/moved for too long
			ppfmt.Infof(pp.EmojiWarning, "Zone %q is %q and thus skipped", name, zone.Status)
			// skip these
		default:
			ppfmt.Warningf(pp.EmojiImpossible, "Zone %q is in an undocumented status %q; please report this at %s",
				name, zone.Status, pp.IssueReportingURL)
			ids = append(ids, zone.ID)
		}
	}

	h.cache.listZones.DeleteExpired()
	h.cache.listZones.Set(name, ids, ttlcache.DefaultTTL)

	return ids, true
}

// ZoneOfDomain finds the active zone ID governing a particular domain.
func (h CloudflareHandle) ZoneOfDomain(ctx context.Context, ppfmt pp.PP, domain domain.Domain) (string, bool) {
	if id := h.cache.zoneOfDomain.Get(domain.DNSNameASCII()); id != nil {
		return id.Value(), true
	}

zoneSearch:
	for s := domain.Split(); s.IsValid(); s = s.Next() {
		zoneName := s.ZoneNameASCII()
		zones, ok := h.ListZones(ctx, ppfmt, zoneName)
		if !ok {
			return "", false
		}

		// The operation went through. No need to perform any sanity checking in near future!
		h.skipSanityCheckToken()

		switch len(zones) {
		case 0: // len(zones) == 0
			continue zoneSearch
		case 1: // len(zones) == 1
			h.cache.zoneOfDomain.DeleteExpired()
			h.cache.zoneOfDomain.Set(domain.DNSNameASCII(), zones[0], ttlcache.DefaultTTL)
			return zones[0], true
		default: // len(zones) > 1
			ppfmt.Warningf(pp.EmojiImpossible,
				"Found multiple active zones named %q (IDs: %s); please report this at %s",
				zoneName, pp.EnglishJoin(zones), pp.IssueReportingURL)
			return "", false
		}
	}

	ppfmt.Warningf(pp.EmojiError, "Failed to find the zone of %q", domain.Describe())

	return "", false
}

// ListRecords calls cloudflare.ListDNSRecords.
func (h CloudflareHandle) ListRecords(ctx context.Context, ppfmt pp.PP,
	domain domain.Domain, ipNet ipnet.Type,
) ([]Record, bool, bool) {
	if rmap := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rmap != nil {
		return *rmap.Value(), true, true
	}

	zone, ok := h.ZoneOfDomain(ctx, ppfmt, domain)
	if !ok {
		return nil, false, false
	}

	// The operation went through. No need to perform any sanity checking in near future!
	h.skipSanityCheckToken()

	//nolint:exhaustruct // Other fields are intentionally unspecified
	raw, _, err := h.cf.ListDNSRecords(ctx,
		cloudflare.ZoneIdentifier(zone),
		cloudflare.ListDNSRecordsParams{
			Name: domain.DNSNameASCII(),
			Type: ipNet.RecordType(),
		})
	if err != nil {
		ppfmt.Warningf(pp.EmojiError,
			"Failed to retrieve %s records of %q: %v",
			ipNet.RecordType(), domain.Describe(), err)
		return nil, false, false
	}

	rs := make([]Record, 0, len(raw))
	for _, r := range raw {
		ip, err := netip.ParseAddr(r.Content)
		if err != nil {
			ppfmt.Warningf(pp.EmojiImpossible,
				"Failed to parse the IP address in an %s record of %q (ID: %s): %v",
				ipNet.RecordType(), domain.Describe(), r.ID, err)
			return nil, false, false
		}

		rs = append(rs, Record{ID: r.ID, IP: ip})
	}

	h.cache.listRecords[ipNet].DeleteExpired()
	h.cache.listRecords[ipNet].Set(domain.DNSNameASCII(), &rs, ttlcache.DefaultTTL)

	return rs, false, true
}

// DeleteRecord calls cloudflare.DeleteDNSRecord.
func (h CloudflareHandle) DeleteRecord(ctx context.Context, ppfmt pp.PP,
	domain domain.Domain, ipNet ipnet.Type, id string,
) bool {
	zone, ok := h.ZoneOfDomain(ctx, ppfmt, domain)
	if !ok {
		return false
	}

	if err := h.cf.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(zone), id); err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to delete a stale %s record of %q (ID: %s): %v",
			ipNet.RecordType(), domain.Describe(), id, err)

		h.cache.listRecords[ipNet].Delete(domain.DNSNameASCII())

		return false
	}

	// The operation went through. No need to perform any sanity checking in near future!
	h.skipSanityCheckToken()

	if rs := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rs != nil {
		*rs.Value() = slices.DeleteFunc(*rs.Value(), func(r Record) bool { return r.ID == id })
	}

	return true
}

// UpdateRecord calls cloudflare.UpdateDNSRecord.
func (h CloudflareHandle) UpdateRecord(ctx context.Context, ppfmt pp.PP,
	domain domain.Domain, ipNet ipnet.Type, id string, ip netip.Addr,
) bool {
	zone, ok := h.ZoneOfDomain(ctx, ppfmt, domain)
	if !ok {
		return false
	}

	//nolint:exhaustruct // Other fields are intentionally omitted
	params := cloudflare.UpdateDNSRecordParams{
		ID:      id,
		Content: ip.String(),
	}

	if _, err := h.cf.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(zone), params); err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to update a stale %s record of %q (ID: %s): %v",
			ipNet.RecordType(), domain.Describe(), id, err)

		h.cache.listRecords[ipNet].Delete(domain.DNSNameASCII())

		return false
	}

	// The operation went through. No need to perform any sanity checking in near future!
	h.skipSanityCheckToken()

	if rs := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rs != nil {
		for i, r := range *rs.Value() {
			if r.ID == id {
				(*rs.Value())[i].IP = ip
			}
		}
	}

	return true
}

// CreateRecord calls cloudflare.CreateDNSRecord.
func (h CloudflareHandle) CreateRecord(ctx context.Context, ppfmt pp.PP,
	domain domain.Domain, ipNet ipnet.Type, ip netip.Addr, ttl TTL, proxied bool, recordComment string,
) (string, bool) {
	zone, ok := h.ZoneOfDomain(ctx, ppfmt, domain)
	if !ok {
		return "", false
	}

	//nolint:exhaustruct // Other fields are intentionally omitted
	params := cloudflare.CreateDNSRecordParams{
		Name:    domain.DNSNameASCII(),
		Type:    ipNet.RecordType(),
		Content: ip.String(),
		TTL:     ttl.Int(),
		Proxied: &proxied,
		Comment: recordComment,
	}

	res, err := h.cf.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zone), params)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to add a new %s record of %q: %v",
			ipNet.RecordType(), domain.Describe(), err)

		h.cache.listRecords[ipNet].Delete(domain.DNSNameASCII())

		return "", false
	}

	// The operation went through. No need to perform any sanity checking in near future!
	h.skipSanityCheckToken()

	if rs := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rs != nil {
		*rs.Value() = append([]Record{{ID: res.ID, IP: ip}}, *rs.Value()...)
	}

	return res.ID, true
}
