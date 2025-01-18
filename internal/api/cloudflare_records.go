package api

import (
	"context"
	"errors"
	"net/netip"
	"slices"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func hintRecordPermission(ppfmt pp.PP, err error) {
	var authentication *cloudflare.AuthenticationError
	var authorization *cloudflare.AuthorizationError
	if errors.As(err, &authentication) || errors.As(err, &authorization) {
		ppfmt.NoticeOncef(pp.MessageRecordPermission, pp.EmojiHint,
			"Double check your API token. "+
				`Make sure you granted the "Edit" permission of "Zone - DNS"`)
	}
}

func hintMismatchedRecordAttributes(ppfmt pp.PP) {
	ppfmt.NoticeOncef(pp.MessageMismatchedRecordAttributes, pp.EmojiHint,
		"The updater will not overwrite proxy statuses, TTLs, or record comments; "+
			"you can change them in your Cloudflare dashboard at https://dash.cloudflare.com",
	)
}

// ListZones returns a list of zone IDs with the zone name.
func (h CloudflareHandle) ListZones(ctx context.Context, ppfmt pp.PP, name string) ([]ID, bool) {
	// WithZoneFilters does not work with the empty zone name,
	// and the owner of the DNS root zone will not be managed by Cloudflare anyways!
	if name == "" {
		return []ID{}, true
	}

	if ids := h.cache.listZones.Get(name); ids != nil {
		return ids.Value(), true
	}

	res, err := h.cf.ListZonesContext(ctx, cloudflare.WithZoneFilters(name, "", ""))
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to check the existence of a zone named %s: %v", name, err)
		hintRecordPermission(ppfmt, err)
		return nil, false
	}

	ids := make([]ID, 0, len(res.Result))
	for _, zone := range res.Result {
		// The list of possible statuses was at https://api.cloudflare.com/#zone-list-zones
		// but the documentation is missing now.
		switch zone.Status {
		case "active": // fully working
			ids = append(ids, ID(zone.ID))
		case
			"deactivated",  // violating term of service, etc.
			"initializing", // the setup was just started?
			"moved",        // domain registrar not pointing to Cloudflare
			"pending":      // the setup was not completed
			ppfmt.Noticef(pp.EmojiWarning, "DNS zone %s is %q in your Cloudflare account; some features (e.g., proxying) might not work as expected", name, zone.Status) //nolint:lll
			ids = append(ids, ID(zone.ID))
		case
			"deleted": // archived, pending/moved for too long
			ppfmt.Infof(pp.EmojiWarning, "DNS zone %s is %q in your Cloudflare account and thus skipped", name, zone.Status)
		default:
			ppfmt.Noticef(pp.EmojiImpossible, "DNS zone %s is in an undocumented status %q in your Cloudflare account; please report this at %s", //nolint:lll
				name, zone.Status, pp.IssueReportingURL)
			ids = append(ids, ID(zone.ID))
		}
	}

	h.cache.listZones.DeleteExpired()
	h.cache.listZones.Set(name, ids, ttlcache.DefaultTTL)

	return ids, true
}

// ZoneIDOfDomain finds the active zone ID governing a particular domain.
func (h CloudflareHandle) ZoneIDOfDomain(ctx context.Context, ppfmt pp.PP, domain domain.Domain) (ID, bool) {
	if id := h.cache.zoneIDOfDomain.Get(domain.DNSNameASCII()); id != nil {
		return id.Value(), true
	}

zoneSearch:
	for zoneName := range domain.Zones {
		zones, ok := h.ListZones(ctx, ppfmt, zoneName)
		if !ok {
			return "", false
		}

		switch len(zones) {
		case 0: // len(zones) == 0
			continue zoneSearch
		case 1: // len(zones) == 1
			h.cache.zoneIDOfDomain.DeleteExpired()
			h.cache.zoneIDOfDomain.Set(domain.DNSNameASCII(), zones[0], ttlcache.DefaultTTL)
			return zones[0], true
		default: // len(zones) > 1
			ppfmt.Noticef(pp.EmojiImpossible,
				"Found multiple active zones named %s (IDs: %s); please report this at %s",
				zoneName, pp.EnglishJoinMap(ID.String, zones), pp.IssueReportingURL)
			return "", false
		}
	}

	ppfmt.Noticef(pp.EmojiError, "Failed to find the zone of %s", domain.Describe())

	return "", false
}

// ListRecords calls cloudflare.ListDNSRecords.
func (h CloudflareHandle) ListRecords(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain,
	expectedParams RecordParams,
) ([]Record, bool, bool) {
	if rmap := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rmap != nil {
		return *rmap.Value(), true, true
	}

	zone, ok := h.ZoneIDOfDomain(ctx, ppfmt, domain)
	if !ok {
		return nil, false, false
	}

	//nolint:exhaustruct // Other fields are intentionally unspecified
	raw, _, err := h.cf.ListDNSRecords(ctx,
		cloudflare.ZoneIdentifier(string(zone)),
		cloudflare.ListDNSRecordsParams{
			Name: domain.DNSNameASCII(),
			Type: ipNet.RecordType(),
		})
	if err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to retrieve %s records of %s: %v",
			ipNet.RecordType(), domain.Describe(), err)
		hintRecordPermission(ppfmt, err)
		return nil, false, false
	}

	rs := make([]Record, 0, len(raw))
	for _, r := range raw {
		id := ID(r.ID)
		ip, err := netip.ParseAddr(r.Content)
		if err != nil {
			ppfmt.Noticef(pp.EmojiImpossible,
				"Failed to parse the IP address in an %s record of %s (ID: %s): %v",
				ipNet.RecordType(), domain.Describe(), id, err)
			return nil, false, false
		}

		if TTL(r.TTL) != expectedParams.TTL {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"The TTL of the %s record of %s (ID: %s) differs from the value of TTL (%s) and will be kept",
				ipNet.RecordType(), domain.Describe(), id, expectedParams.TTL.Describe(),
			)
			hintMismatchedRecordAttributes(ppfmt)
		}
		// by default, proxied = false
		if r.Proxied == nil && expectedParams.Proxied ||
			r.Proxied != nil && *r.Proxied != expectedParams.Proxied {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"The proxy status of the %s record of %s (ID: %s) differs from the value of PROXIED (%v for this domain) and will be kept", //nolint:lll
				ipNet.RecordType(), domain.Describe(), id, expectedParams.Proxied,
			)
			hintMismatchedRecordAttributes(ppfmt)
		}
		if r.Comment != expectedParams.Comment {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"The comment of the %s record of %s (ID: %s) differs from the value of RECORD_COMMENT (%q) and will be kept", //nolint:lll
				ipNet.RecordType(), domain.Describe(), id, expectedParams.Comment,
			)
			hintMismatchedRecordAttributes(ppfmt)
		}

		rs = append(rs, Record{
			ID: ID(r.ID),
			IP: ip,
			RecordParams: RecordParams{
				TTL:     TTL(r.TTL),
				Proxied: r.Proxied != nil && *r.Proxied,
				Comment: r.Comment,
			},
		})
	}

	h.cache.listRecords[ipNet].DeleteExpired()
	h.cache.listRecords[ipNet].Set(domain.DNSNameASCII(), &rs, ttlcache.DefaultTTL)

	return rs, false, true
}

// DeleteRecord calls cloudflare.DeleteDNSRecord.
func (h CloudflareHandle) DeleteRecord(ctx context.Context, ppfmt pp.PP,
	ipNet ipnet.Type, domain domain.Domain, id ID,
	mode DeletionMode,
) bool {
	zone, ok := h.ZoneIDOfDomain(ctx, ppfmt, domain)
	if !ok {
		return false
	}

	if err := h.cf.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(string(zone)), string(id)); err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to delete a stale %s record of %s (ID: %s): %v",
			ipNet.RecordType(), domain.Describe(), id, err)
		hintRecordPermission(ppfmt, err)
		if mode == RegularDelitionMode {
			h.cache.listRecords[ipNet].Delete(domain.DNSNameASCII())
		}
		return false
	}

	if rs := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rs != nil {
		*rs.Value() = slices.DeleteFunc(*rs.Value(), func(r Record) bool { return r.ID == id })
	}

	return true
}

// UpdateRecord calls cloudflare.UpdateDNSRecord.
func (h CloudflareHandle) UpdateRecord(ctx context.Context, ppfmt pp.PP,
	ipNet ipnet.Type, domain domain.Domain, id ID, ip netip.Addr,
	currentParams, expectedParams RecordParams,
) bool {
	zone, ok := h.ZoneIDOfDomain(ctx, ppfmt, domain)
	if !ok {
		return false
	}

	//nolint:exhaustruct // Other fields are intentionally omitted
	params := cloudflare.UpdateDNSRecordParams{
		ID:      string(id),
		Content: ip.String(),
	}

	resp, err := h.cf.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(string(zone)), params)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to update a stale %s record of %s (ID: %s): %v",
			ipNet.RecordType(), domain.Describe(), id, err)
		hintRecordPermission(ppfmt, err)

		h.cache.listRecords[ipNet].Delete(domain.DNSNameASCII())

		return false
	}

	if TTL(resp.TTL) != currentParams.TTL && TTL(resp.TTL) != expectedParams.TTL {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"The TTL of the %s record of %s (ID: %s) differs from the value of TTL (%s) and will be kept",
			ipNet.RecordType(), domain.Describe(), ip, expectedParams.TTL.Describe(),
		)
		hintMismatchedRecordAttributes(ppfmt)
	}
	// by default, proxied = false
	if resp.Proxied == nil && currentParams.Proxied && expectedParams.Proxied ||
		resp.Proxied != nil && *resp.Proxied != currentParams.Proxied && *resp.Proxied != expectedParams.Proxied {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"The proxy status of the %s record of %s (ID: %s) differs from the value of PROXIED (%v for this domain) and will be kept", //nolint:lll
			ipNet.RecordType(), domain.Describe(), id, expectedParams.Proxied,
		)
		hintMismatchedRecordAttributes(ppfmt)
	}
	if resp.Comment != currentParams.Comment && resp.Comment != expectedParams.Comment {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"The comment of the %s record of %s (ID: %s) differs from the value of RECORD_COMMENT (%q) and will be kept", //nolint:lll
			ipNet.RecordType(), domain.Describe(), id, expectedParams.Comment,
		)
		hintMismatchedRecordAttributes(ppfmt)
	}

	if rs := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rs != nil {
		for i, r := range *rs.Value() {
			if r.ID == id {
				(*rs.Value())[i] = Record{
					ID: id,
					IP: ip,
					RecordParams: RecordParams{
						TTL:     TTL(resp.TTL),
						Proxied: resp.Proxied != nil && *resp.Proxied,
						Comment: resp.Comment,
					},
				}
			}
		}
	}

	return true
}

// CreateRecord calls cloudflare.CreateDNSRecord.
func (h CloudflareHandle) CreateRecord(ctx context.Context, ppfmt pp.PP,
	ipNet ipnet.Type, domain domain.Domain, ip netip.Addr, params RecordParams,
) (ID, bool) {
	zone, ok := h.ZoneIDOfDomain(ctx, ppfmt, domain)
	if !ok {
		return "", false
	}

	//nolint:exhaustruct // Other fields are intentionally omitted
	ps := cloudflare.CreateDNSRecordParams{
		Name:    domain.DNSNameASCII(),
		Type:    ipNet.RecordType(),
		Content: ip.String(),
		TTL:     params.TTL.Int(),
		Proxied: &params.Proxied,
		Comment: params.Comment,
	}

	res, err := h.cf.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(string(zone)), ps)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to add a new %s record of %s: %v",
			ipNet.RecordType(), domain.Describe(), err)
		hintRecordPermission(ppfmt, err)

		h.cache.listRecords[ipNet].Delete(domain.DNSNameASCII())

		return "", false
	}

	if rs := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rs != nil {
		*rs.Value() = append([]Record{{ID: ID(res.ID), IP: ip, RecordParams: params}}, *rs.Value()...)
	}

	return ID(res.ID), true
}
