package api

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strconv"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/jellydator/ttlcache/v3"

	apitags "github.com/favonia/cloudflare-ddns/internal/api/tags"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type zoneMeta struct {
	ID        ID
	AccountID ID
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

func hintMismatchedTTL(
	ppfmt pp.PP,
	ipFamily ipnet.Family,
	domain domain.Domain,
	id ID,
	dashboardURL string,
	current, target TTL,
) {
	ppfmt.Noticef(pp.EmojiUserWarning,
		"The TTL for the %s record of %s (ID: %s) is %s. However, the preferred TTL is %s. You can either change the TTL to %s in the Cloudflare dashboard at %s or change the preferred TTL with TTL=%d.", //nolint:lll
		ipFamily.RecordType(), domain.Describe(), id,
		current.Describe(), target.Describe(), target.Describe(), dashboardURL, current.Int(),
	)
}

func hintMismatchedProxied(
	ppfmt pp.PP,
	ipFamily ipnet.Family,
	domain domain.Domain,
	id ID,
	dashboardURL string,
	current, target bool,
) {
	descriptions := map[bool]string{
		true:  "proxied",
		false: "not proxied (DNS only)",
	}

	ppfmt.Noticef(pp.EmojiUserWarning,
		`The %s record of %s (ID: %s) is %s. However, the preferred proxy setting is %s. You can either change the proxy status to "%s" in the Cloudflare dashboard at %s or change the value of PROXIED to match the current setting.`, //nolint:lll
		ipFamily.RecordType(), domain.Describe(), id,
		descriptions[current], descriptions[target], descriptions[target], dashboardURL,
	)
}

func hintMismatchedComment(
	ppfmt pp.PP,
	ipFamily ipnet.Family,
	domain domain.Domain,
	id ID,
	dashboardURL string,
	current, target string,
) {
	ppfmt.Noticef(pp.EmojiUserWarning,
		`The comment for %s record of %s (ID: %s) is %s. However, the preferred comment is %s. You can either change the comment in the Cloudflare dashboard at %s or change the value of RECORD_COMMENT to match the current comment.`, //nolint:lll
		ipFamily.RecordType(), domain.Describe(), id,
		describeFreeFormString(current), describeFreeFormString(target), dashboardURL,
	)
}

func hintUndocumentedTags(ppfmt pp.PP, ipFamily ipnet.Family, domain domain.Domain, id ID, tags []string) {
	if len(tags) == 0 {
		return
	}
	ppfmt.Noticef(pp.EmojiImpossible,
		"Found tags %s in an %s record of %s (ID: %s) that are not in Cloudflare's documented name:value form; this should not happen and please report this at %s", //nolint:lll
		pp.EnglishJoinMap(strconv.Quote, tags), ipFamily.RecordType(), domain.Describe(), id, pp.IssueReportingURL,
	)
}

func newUndocumentedTags(returned, requested []string) []string {
	newUndocumented := make([]string, 0)
	for _, tag := range apitags.Undocumented(returned) {
		if slices.Contains(requested, tag) {
			continue
		}
		newUndocumented = append(newUndocumented, tag)
	}
	return newUndocumented
}

func (h cloudflareHandle) listZoneMeta(ctx context.Context, ppfmt pp.PP, name string) ([]zoneMeta, bool) {
	// WithZoneFilters does not work with the empty zone name,
	// and the owner of the DNS root zone will not be managed by Cloudflare anyways!
	if name == "" {
		return []zoneMeta{}, true
	}

	if zones := h.cache.listZones.Get(name); zones != nil {
		return zones.Value(), true
	}

	res, err := h.cf.ListZonesContext(ctx, cloudflare.WithZoneFilters(name, "", ""))
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to check the existence of a zone named %s: %v", name, err)
		hintRecordPermission(ppfmt, err)
		return nil, false
	}

	zones := make([]zoneMeta, 0, len(res.Result))
	for _, zone := range res.Result {
		ref := zoneMeta{ID: ID(zone.ID), AccountID: ID(zone.Account.ID)}
		// Cloudflare documents zone-status names again at
		// https://developers.cloudflare.com/dns/zone-setups/reference/domain-status/ .
		// Keep a default branch in case the API surfaces additional or
		// older statuses beyond the current docs.
		switch zone.Status {
		case "active": // fully working
			zones = append(zones, ref)
		case
			"deactivated",  // violating term of service, etc.
			"initializing", // the setup was just started?
			"moved",        // domain registrar not pointing to Cloudflare
			"pending":      // the setup was not completed
			ppfmt.Noticef(pp.EmojiWarning, "DNS zone %s is %q in your Cloudflare account; some features (e.g., proxying) might not work as expected", name, zone.Status) //nolint:lll
			zones = append(zones, ref)
		case
			"deleted": // archived, pending/moved for too long
			ppfmt.Infof(pp.EmojiWarning, "DNS zone %s is %q in your Cloudflare account and thus skipped", name, zone.Status)
		default:
			ppfmt.Noticef(pp.EmojiImpossible, "DNS zone %s is in an undocumented status %q in your Cloudflare account; please report this at %s", //nolint:lll
				name, zone.Status, pp.IssueReportingURL)
			zones = append(zones, ref)
		}
	}

	h.cache.listZones.DeleteExpired()
	h.cache.listZones.Set(name, zones, ttlcache.DefaultTTL)

	return zones, true
}

// listZones returns a list of zone IDs with the zone name.
func (h cloudflareHandle) listZones(ctx context.Context, ppfmt pp.PP, name string) ([]ID, bool) {
	zones, ok := h.listZoneMeta(ctx, ppfmt, name)
	if !ok {
		return nil, false
	}

	ids := make([]ID, 0, len(zones))
	for _, zone := range zones {
		ids = append(ids, zone.ID)
	}
	return ids, true
}

func (h cloudflareHandle) zoneMetaOfDomain(ctx context.Context, ppfmt pp.PP, domain domain.Domain) (zoneMeta, bool) {
	var zero zoneMeta

	if zone := h.cache.zoneOfDomain.Get(domain.DNSNameASCII()); zone != nil {
		return zone.Value(), true
	}

zoneSearch:
	for zoneName := range domain.Zones {
		zones, ok := h.listZoneMeta(ctx, ppfmt, zoneName)
		if !ok {
			return zero, false
		}

		switch len(zones) {
		case 0: // len(zones) == 0
			continue zoneSearch
		case 1: // len(zones) == 1
			h.cache.zoneOfDomain.DeleteExpired()
			h.cache.zoneOfDomain.Set(domain.DNSNameASCII(), zones[0], ttlcache.DefaultTTL)
			return zones[0], true
		default: // len(zones) > 1
			ids := make([]ID, 0, len(zones))
			for _, zone := range zones {
				ids = append(ids, zone.ID)
			}
			ppfmt.Noticef(pp.EmojiImpossible,
				"Found multiple active zones named %s (IDs: %s); please report this at %s",
				zoneName, pp.EnglishJoinMap(ID.String, ids), pp.IssueReportingURL)
			// The suffix walk reached a semantic dead end at zoneName. Retry the
			// traversed suffixes up to and including that boundary next cycle.
			for cachedZoneName := range domain.Zones {
				h.cache.listZones.Delete(cachedZoneName)
				if cachedZoneName == zoneName {
					break
				}
			}
			return zero, false
		}
	}

	// The suffix walk found no usable zone. Clear all candidate suffix caches so
	// new or recovered zones are retried on the next cycle instead of waiting for
	// the full zone-list cache TTL.
	for zoneName := range domain.Zones {
		h.cache.listZones.Delete(zoneName)
	}

	ppfmt.Noticef(pp.EmojiError, "Failed to find the zone of %s; will try again", domain.Describe())

	return zero, false
}

// zoneIDOfDomain finds the active zone ID governing a particular domain.
func (h cloudflareHandle) zoneIDOfDomain(ctx context.Context, ppfmt pp.PP, domain domain.Domain) (ID, bool) {
	zone, ok := h.zoneMetaOfDomain(ctx, ppfmt, domain)
	if !ok {
		return "", false
	}
	return zone.ID, true
}

// ListRecords calls cloudflare.ListDNSRecords.
func (h cloudflareHandle) ListRecords(ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, domain domain.Domain,
	fallbackParams RecordParams,
) ([]Record, bool, bool) {
	if cachedManagedRecords := h.cache.listRecords[ipFamily].Get(domain.DNSNameASCII()); cachedManagedRecords != nil {
		// Cache stores managed records only; this assumes a stable selector per handle.
		return *cachedManagedRecords.Value(), true, true
	}

	zone, ok := h.zoneMetaOfDomain(ctx, ppfmt, domain)
	if !ok {
		return nil, false, false
	}
	dashboardURL := cloudflareDNSRecordsDeeplink(zone.AccountID, zone.ID)

	raw, _, err := h.cf.ListDNSRecords(ctx,
		cloudflare.ZoneIdentifier(string(zone.ID)),
		//nolint:exhaustruct // Query params intentionally set only fields used by the selector.
		cloudflare.ListDNSRecordsParams{
			Type: ipFamily.RecordType(),
			Name: domain.DNSNameASCII(),
		})
	if err != nil {
		ppfmt.Noticef(pp.EmojiError,
			"Failed to retrieve %s records of %s: %v",
			ipFamily.RecordType(), domain.Describe(), err)
		hintRecordPermission(ppfmt, err)
		return nil, false, false
	}

	managedRecords := make([]Record, 0, len(raw))
	for _, rawRecord := range raw {
		if !h.options.MatchManagedRecordComment(rawRecord.Comment) {
			continue
		}

		id := ID(rawRecord.ID)
		ip, err := netip.ParseAddr(rawRecord.Content)
		if err != nil {
			ppfmt.Noticef(pp.EmojiImpossible,
				"Failed to parse the IP address in an %s record of %s (ID: %s): %v",
				ipFamily.RecordType(), domain.Describe(), id, err)
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
		hintUndocumentedTags(ppfmt, ipFamily, domain, id, apitags.Undocumented(record.Tags))
		managedRecords = append(managedRecords, record)

		if record.TTL != fallbackParams.TTL {
			hintMismatchedTTL(ppfmt, ipFamily, domain, id, dashboardURL, record.TTL, fallbackParams.TTL)
		}
		if record.Proxied != fallbackParams.Proxied {
			hintMismatchedProxied(ppfmt, ipFamily, domain, id, dashboardURL, record.Proxied, fallbackParams.Proxied)
		}
		if record.Comment != fallbackParams.Comment {
			hintMismatchedComment(ppfmt, ipFamily, domain, id, dashboardURL, record.Comment, fallbackParams.Comment)
		}
	}

	h.cache.listRecords[ipFamily].DeleteExpired()
	h.cache.listRecords[ipFamily].Set(domain.DNSNameASCII(), &managedRecords, ttlcache.DefaultTTL)

	return managedRecords, false, true
}

// DeleteRecord calls cloudflare.DeleteDNSRecord.
func (h cloudflareHandle) DeleteRecord(ctx context.Context, ppfmt pp.PP,
	ipFamily ipnet.Family, domain domain.Domain, id ID,
	mode DeletionMode,
) bool {
	zone, ok := h.zoneMetaOfDomain(ctx, ppfmt, domain)
	if !ok {
		return false
	}

	if err := h.cf.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(string(zone.ID)), string(id)); err != nil {
		ppfmt.Noticef(pp.EmojiError, "Could not confirm deletion of outdated %s record of %s (ID: %s): %v",
			ipFamily.RecordType(), domain.Describe(), id, err)
		hintRecordPermission(ppfmt, err)
		if mode == RegularDeletionMode {
			h.cache.listRecords[ipFamily].Delete(domain.DNSNameASCII())
		}
		return false
	}

	if rs := h.cache.listRecords[ipFamily].Get(domain.DNSNameASCII()); rs != nil {
		*rs.Value() = slices.DeleteFunc(*rs.Value(), func(r Record) bool { return r.ID == id })
	}

	return true
}

// UpdateRecord calls cloudflare.UpdateDNSRecord.
func (h cloudflareHandle) UpdateRecord(ctx context.Context, ppfmt pp.PP,
	ipFamily ipnet.Family, domain domain.Domain, id ID, ip netip.Addr,
	desiredParams RecordParams,
) bool {
	zone, ok := h.zoneMetaOfDomain(ctx, ppfmt, domain)
	if !ok {
		return false
	}
	dashboardURL := cloudflareDNSRecordsDeeplink(zone.AccountID, zone.ID)

	// Keep this mutating request literal exhaustive (do not add //nolint:exhaustruct):
	// - Reconciled-on-update fields: type/name/content + desired metadata
	//   (ttl/proxied/comment/tags).
	// - Cloudflare API docs (edit DNS record):
	//   https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/edit/
	// - For Cloudflare's UpdateDNSRecord API, nil comment means "keep current".
	//   To reconcile comments (including empty), we must always pass a pointer.
	// - Tags are always sent so tag clearing can be expressed explicitly.
	// - Remaining request fields are server-determined for the record kinds
	//   handled here and are intentionally set to their zero forms.
	// Exhaustiveness ensures upstream API field additions are reviewed explicitly.
	desiredComment := desiredParams.Comment
	desiredTags := slices.Clone(desiredParams.Tags)
	if desiredTags == nil {
		desiredTags = []string{}
	}
	updateRequestParams := cloudflare.UpdateDNSRecordParams{
		Type:    ipFamily.RecordType(), // managed: A/AAAA type is part of desired record identity.
		Name:    domain.DNSNameASCII(), // managed: canonical fqdn identity for this reconciler unit.
		Content: ip.String(),           // managed: desired IP address.
		// server-determined for this reconciler: we only manage A/AAAA records here.
		// Cloudflare uses Data for other record kinds (for example SRV/LOC), so we keep nil.
		Data: nil,
		ID:   string(id), // managed: target record identifier in API route/body.
		// server-determined for this reconciler: Priority applies to other record kinds
		// (for example MX/SRV/URI), not A/AAAA.
		Priority: nil,
		TTL:      desiredParams.TTL.Int(), // managed: desired TTL.
		Proxied:  &desiredParams.Proxied,  // managed: desired proxy mode.
		Comment:  &desiredComment,         // managed: desired comment (including explicit empty).
		Tags:     desiredTags,             // managed: desired tags (always sent to allow clearing).
		Settings: cloudflare.DNSRecordSettings{
			// server-determined for this reconciler: per-record CNAME flattening is
			// CNAME-specific and not managed for A/AAAA.
			FlattenCNAME: nil,
		},
	}

	r, err := h.cf.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(string(zone.ID)), updateRequestParams)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Could not confirm update of outdated %s record of %s (ID: %s): %v",
			ipFamily.RecordType(), domain.Describe(), id, err)
		hintRecordPermission(ppfmt, err)

		h.cache.listRecords[ipFamily].Delete(domain.DNSNameASCII())

		return false
	}

	if TTL(r.TTL) != desiredParams.TTL {
		hintMismatchedTTL(ppfmt, ipFamily, domain, id, dashboardURL, TTL(r.TTL), desiredParams.TTL)
	}
	updatedProxied := r.Proxied != nil && *r.Proxied // by default, proxied = false
	if updatedProxied != desiredParams.Proxied {
		hintMismatchedProxied(ppfmt, ipFamily, domain, id, dashboardURL, updatedProxied, desiredParams.Proxied)
	}
	if r.Comment != desiredParams.Comment {
		hintMismatchedComment(ppfmt, ipFamily, domain, id, dashboardURL, r.Comment, desiredParams.Comment)
	}

	currentParams := RecordParams{
		TTL:     TTL(r.TTL),
		Proxied: updatedProxied,
		Comment: r.Comment,
		Tags:    r.Tags,
	}
	hintUndocumentedTags(ppfmt, ipFamily, domain, id, newUndocumentedTags(currentParams.Tags, desiredParams.Tags))

	if rs := h.cache.listRecords[ipFamily].Get(domain.DNSNameASCII()); rs != nil {
		if !h.options.MatchManagedRecordComment(currentParams.Comment) {
			*rs.Value() = slices.DeleteFunc(*rs.Value(), func(r Record) bool { return r.ID == id })
			return true
		}

		updatedRecord := Record{
			ID:           id,
			IP:           ip,
			RecordParams: currentParams,
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
func (h cloudflareHandle) CreateRecord(ctx context.Context, ppfmt pp.PP,
	ipFamily ipnet.Family, domain domain.Domain, ip netip.Addr, desiredParams RecordParams,
) (ID, bool) {
	zone, ok := h.zoneMetaOfDomain(ctx, ppfmt, domain)
	if !ok {
		return "", false
	}

	createRequestParams := cloudflare.CreateDNSRecordParams{
		// Cloudflare API docs (create DNS record):
		// https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/create/
		// server-determined: create timestamp is assigned by Cloudflare.
		CreatedOn: time.Time{},
		// server-determined: modified timestamp is assigned by Cloudflare.
		ModifiedOn: time.Time{},
		Type:       ipFamily.RecordType(), // managed: A/AAAA type in desired identity.
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
		TTL:      desiredParams.TTL.Int(), // managed: desired TTL.
		Proxied:  &desiredParams.Proxied,  // managed: desired proxy mode.
		// server-determined: capability flag returned by Cloudflare, not a desired input.
		Proxiable: false,
		Comment:   desiredParams.Comment, // managed: desired comment.
		Tags:      desiredParams.Tags,    // managed: desired tags.
		Settings: cloudflare.DNSRecordSettings{
			// server-determined for this reconciler: per-record CNAME flattening is
			// not managed for A/AAAA.
			FlattenCNAME: nil,
		},
	}

	res, err := h.cf.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(string(zone.ID)), createRequestParams)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Could not confirm creation of new %s record of %s: %v",
			ipFamily.RecordType(), domain.Describe(), err)
		hintRecordPermission(ppfmt, err)

		h.cache.listRecords[ipFamily].Delete(domain.DNSNameASCII())

		return "", false
	}

	if rs := h.cache.listRecords[ipFamily].Get(domain.DNSNameASCII()); rs != nil &&
		h.options.MatchManagedRecordComment(desiredParams.Comment) {
		*rs.Value() = append([]Record{{ID: ID(res.ID), IP: ip, RecordParams: desiredParams}}, *rs.Value()...)
	}

	hintUndocumentedTags(ppfmt, ipFamily, domain, ID(res.ID), newUndocumentedTags(res.Tags, desiredParams.Tags))

	return ID(res.ID), true
}
