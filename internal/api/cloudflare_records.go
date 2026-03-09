package api

import (
	"context"
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func matchManagedRecordComment(regex *regexp.Regexp, comment string) bool {
	if regex == nil {
		return true
	}
	return regex.MatchString(comment)
}

func hintRecordPermission(ppfmt pp.PP, err error) {
	var authentication *cloudflare.AuthenticationError
	var authorization *cloudflare.AuthorizationError
	if errors.As(err, &authentication) || errors.As(err, &authorization) {
		ppfmt.NoticeOncef(pp.MessageRecordPermission, pp.EmojiHint,
			"Double check your API token. "+
				`Make sure you granted the "Edit" permission of "Zone - DNS"`)
	}
}

func hintMismatchedTTL(ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain, id ID, current, expected TTL) {
	ppfmt.Noticef(pp.EmojiUserWarning,
		"The TTL for the %s record of %s (ID: %s) is %s. However, it is expected to be %s. You can either change the TTL to %s in the Cloudflare dashboard at https://dash.cloudflare.com or change the expected TTL with TTL=%d.", //nolint:lll
		ipNet.RecordType(), domain.Describe(), id,
		current.Describe(), expected.Describe(), expected.Describe(), current.Int(),
	)
}

func hintMismatchedProxied(ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain, id ID, current, expected bool) {
	descriptions := map[bool]string{
		true:  "proxied",
		false: "not proxied (DNS only)",
	}
	negation := map[bool]string{
		true:  "",
		false: "not ",
	}

	ppfmt.Noticef(pp.EmojiUserWarning,
		`The %s record of %s (ID: %s) is %s. However, it is %sexpected to be proxied. You can either change the proxy status to "%s" in the Cloudflare dashboard at https://dash.cloudflare.com or change the value of PROXIED to match the current setting.`, //nolint:lll
		ipNet.RecordType(), domain.Describe(), id,
		descriptions[current], negation[expected], descriptions[expected],
	)
}

func hintMismatchedComment(ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain, id ID, current, expected string) {
	ppfmt.Noticef(pp.EmojiUserWarning,
		`The comment for %s record of %s (ID: %s) is %s. However, it is expected to be %s. You can either change the comment in the Cloudflare dashboard at https://dash.cloudflare.com or change the value of RECORD_COMMENT to match the current comment.`, //nolint:lll
		ipNet.RecordType(), domain.Describe(), id, DescribeFreeFormString(current), DescribeFreeFormString(expected),
	)
}

func hintMismatchedTags(ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain, id ID, current, expected []string) {
	ppfmt.Noticef(pp.EmojiUserWarning,
		"The tags for the %s record of %s (ID: %s) are %q. However, they are expected to be %q. You can either change the tags in the Cloudflare dashboard at https://dash.cloudflare.com or change the value of TAGS to match the current tags.", //nolint:lll
		ipNet.RecordType(), domain.Describe(), id, current, expected,
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
	if cachedManagedRecords := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); cachedManagedRecords != nil {
		// Cache stores managed records only; this assumes a stable selector per handle.
		return *cachedManagedRecords.Value(), true, true
	}

	zone, ok := h.ZoneIDOfDomain(ctx, ppfmt, domain)
	if !ok {
		return nil, false, false
	}

	raw, _, err := h.cf.ListDNSRecords(ctx,
		cloudflare.ZoneIdentifier(string(zone)),
		//nolint:exhaustruct // Query params intentionally set only fields used by the selector.
		cloudflare.ListDNSRecordsParams{
			Type: ipNet.RecordType(),
			Name: domain.DNSNameASCII(),
		})
	if err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to retrieve %s records of %s: %v",
			ipNet.RecordType(), domain.Describe(), err)
		hintRecordPermission(ppfmt, err)
		return nil, false, false
	}

	managedRecords := make([]Record, 0, len(raw))
	for _, rawRecord := range raw {
		if !matchManagedRecordComment(h.options.ManagedRecordsCommentRegex, rawRecord.Comment) {
			continue
		}

		id := ID(rawRecord.ID)
		ip, err := netip.ParseAddr(rawRecord.Content)
		if err != nil {
			ppfmt.Noticef(pp.EmojiImpossible,
				"Failed to parse the IP address in an %s record of %s (ID: %s): %v",
				ipNet.RecordType(), domain.Describe(), id, err)
			return nil, false, false
		}

		record := Record{
			ID: ID(rawRecord.ID),
			IP: ip,
			RecordParams: RecordParams{
				TTL:     TTL(rawRecord.TTL),
				Proxied: rawRecord.Proxied != nil && *rawRecord.Proxied, // by default, proxied = false
				Comment: rawRecord.Comment,
				Tags:    rawRecord.Tags,
			},
		}
		managedRecords = append(managedRecords, record)

		if record.TTL != expectedParams.TTL {
			hintMismatchedTTL(ppfmt, ipNet, domain, id, record.TTL, expectedParams.TTL)
		}
		if record.Proxied != expectedParams.Proxied {
			hintMismatchedProxied(ppfmt, ipNet, domain, id, record.Proxied, expectedParams.Proxied)
		}
		if record.Comment != expectedParams.Comment {
			hintMismatchedComment(ppfmt, ipNet, domain, id, record.Comment, expectedParams.Comment)
		}
	}

	h.cache.listRecords[ipNet].DeleteExpired()
	h.cache.listRecords[ipNet].Set(domain.DNSNameASCII(), &managedRecords, ttlcache.DefaultTTL)

	return managedRecords, false, true
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
		ppfmt.Noticef(pp.EmojiError, "Could not confirm deletion of stale %s record of %s (ID: %s): %v",
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
	// currentParams is kept in the interface as diagnostic context.
	// Desired-state mutation must come from expectedParams.
	_ = currentParams

	zone, ok := h.ZoneIDOfDomain(ctx, ppfmt, domain)
	if !ok {
		return false
	}

	// Keep this mutating request literal exhaustive (do not add //nolint:exhaustruct):
	// - Reconciled-on-update fields: type/name/content + expected metadata
	//   (ttl/proxied/comment/tags).
	// - Cloudflare API docs (edit DNS record):
	//   https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/edit/
	// - For Cloudflare's UpdateDNSRecord API, nil comment means "keep current".
	//   To reconcile comments (including empty), we must always pass a pointer.
	// - Tags are always sent so tag clearing can be expressed explicitly.
	// - Remaining request fields are server-determined for the record kinds
	//   handled here and are intentionally set to their zero forms.
	// Exhaustiveness ensures upstream API field additions are reviewed explicitly.
	expectedComment := expectedParams.Comment
	expectedTags := slices.Clone(expectedParams.Tags)
	if expectedTags == nil {
		expectedTags = []string{}
	}
	params := cloudflare.UpdateDNSRecordParams{
		Type:    ipNet.RecordType(),    // managed: A/AAAA type is part of desired record identity.
		Name:    domain.DNSNameASCII(), // managed: canonical fqdn identity for this reconciler unit.
		Content: ip.String(),           // managed: desired IP address.
		// server-determined for this reconciler: we only manage A/AAAA records here.
		// Cloudflare uses Data for other record kinds (for example SRV/LOC), so we keep nil.
		Data: nil,
		ID:   string(id), // managed: target record identifier in API route/body.
		// server-determined for this reconciler: Priority applies to other record kinds
		// (for example MX/SRV/URI), not A/AAAA.
		Priority: nil,
		TTL:      expectedParams.TTL.Int(), // managed: desired TTL.
		Proxied:  &expectedParams.Proxied,  // managed: desired proxy mode.
		Comment:  &expectedComment,         // managed: desired comment (including explicit empty).
		Tags:     expectedTags,             // managed: desired tags (always sent to allow clearing).
		Settings: cloudflare.DNSRecordSettings{
			// server-determined for this reconciler: per-record CNAME flattening is
			// CNAME-specific and not managed for A/AAAA.
			FlattenCNAME: nil,
		},
	}

	r, err := h.cf.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(string(zone)), params)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Could not confirm update of stale %s record of %s (ID: %s): %v",
			ipNet.RecordType(), domain.Describe(), id, err)
		hintRecordPermission(ppfmt, err)

		h.cache.listRecords[ipNet].Delete(domain.DNSNameASCII())

		return false
	}

	if TTL(r.TTL) != expectedParams.TTL {
		hintMismatchedTTL(ppfmt, ipNet, domain, id, TTL(r.TTL), expectedParams.TTL)
	}
	updatedProxied := r.Proxied != nil && *r.Proxied // by default, proxied = false
	if updatedProxied != expectedParams.Proxied {
		hintMismatchedProxied(ppfmt, ipNet, domain, id, updatedProxied, expectedParams.Proxied)
	}
	if r.Comment != expectedParams.Comment {
		hintMismatchedComment(ppfmt, ipNet, domain, id, r.Comment, expectedParams.Comment)
	}
	if !slices.Equal(r.Tags, expectedParams.Tags) {
		hintMismatchedTags(ppfmt, ipNet, domain, id, r.Tags, expectedParams.Tags)
	}

	updatedParams := RecordParams{
		TTL:     TTL(r.TTL),
		Proxied: updatedProxied,
		Comment: r.Comment,
		Tags:    r.Tags,
	}

	if rs := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rs != nil {
		if !matchManagedRecordComment(h.options.ManagedRecordsCommentRegex, updatedParams.Comment) {
			*rs.Value() = slices.DeleteFunc(*rs.Value(), func(r Record) bool { return r.ID == id })
			return true
		}

		updatedRecord := Record{
			ID:           id,
			IP:           ip,
			RecordParams: updatedParams,
		}
		for i, record := range *rs.Value() {
			if record.ID == id {
				(*rs.Value())[i] = updatedRecord
				return true
			}
		}
		*rs.Value() = append([]Record{updatedRecord}, *rs.Value()...)
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

	ps := cloudflare.CreateDNSRecordParams{
		// Cloudflare API docs (create DNS record):
		// https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/create/
		// server-determined: create timestamp is assigned by Cloudflare.
		CreatedOn: time.Time{},
		// server-determined: modified timestamp is assigned by Cloudflare.
		ModifiedOn: time.Time{},
		Type:       ipNet.RecordType(),    // managed: A/AAAA type in desired identity.
		Name:       domain.DNSNameASCII(), // managed: canonical fqdn.
		Content:    ip.String(),           // managed: desired IP address.
		// server-determined: Meta is Cloudflare-owned metadata in responses.
		Meta: nil,
		// server-determined for this reconciler: Data is for non-A/AAAA record kinds.
		Data: nil,
		// server-determined: record ID is allocated by Cloudflare on create.
		ID: "",
		// server-determined for this reconciler: Priority is for non-A/AAAA kinds.
		Priority: nil,
		TTL:      params.TTL.Int(), // managed: desired TTL.
		Proxied:  &params.Proxied,  // managed: desired proxy mode.
		// server-determined: capability flag returned by Cloudflare, not a desired input.
		Proxiable: false,
		Comment:   params.Comment, // managed: desired comment.
		Tags:      params.Tags,    // managed: desired tags.
		Settings: cloudflare.DNSRecordSettings{
			// server-determined for this reconciler: per-record CNAME flattening is
			// not managed for A/AAAA.
			FlattenCNAME: nil,
		},
	}

	res, err := h.cf.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(string(zone)), ps)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Could not confirm creation of new %s record of %s: %v",
			ipNet.RecordType(), domain.Describe(), err)
		hintRecordPermission(ppfmt, err)

		h.cache.listRecords[ipNet].Delete(domain.DNSNameASCII())

		return "", false
	}

	if rs := h.cache.listRecords[ipNet].Get(domain.DNSNameASCII()); rs != nil &&
		matchManagedRecordComment(h.options.ManagedRecordsCommentRegex, params.Comment) {
		*rs.Value() = append([]Record{{ID: ID(res.ID), IP: ip, RecordParams: params}}, *rs.Value()...)
	}

	return ID(res.ID), true
}
