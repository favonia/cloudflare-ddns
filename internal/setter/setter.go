package setter

import (
	"context"
	"net/netip"
	"sort"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type setter struct {
	Handle  api.Handle
	TTL     api.TTL
	Proxied bool
}

func splitRecords(rmap map[string]netip.Addr, target netip.Addr) (matchedIDs, unmatchedIDs []string) {
	if target.IsValid() {
		for id, ip := range rmap {
			if ip == target {
				matchedIDs = append(matchedIDs, id)
			} else {
				unmatchedIDs = append(unmatchedIDs, id)
			}
		}
	} else {
		for id := range rmap {
			unmatchedIDs = append(unmatchedIDs, id)
		}
	}

	// This is to make Do deterministic so that this package is easier to test.
	// Otherwise, sorting is not needed. The performance penality should be small
	// because in most cases the total number of (matched and unmached) records
	// would be zero or one.
	sort.Strings(matchedIDs)
	sort.Strings(unmatchedIDs)

	return matchedIDs, unmatchedIDs
}

func New(_ppfmt pp.PP, handle api.Handle, ttl api.TTL, proxied bool) (Setter, bool) {
	return &setter{
		Handle:  handle,
		TTL:     ttl,
		Proxied: proxied,
	}, true
}

//nolint: funlen,cyclop,gocognit
func (s *setter) Set(ctx context.Context, ppfmt pp.PP, domain api.Domain, ipnet ipnet.Type, ip netip.Addr) bool { //nolint: lll
	recordType := ipnet.RecordType()
	domainDescription := domain.Describe()

	rs, ok := s.Handle.ListRecords(ctx, ppfmt, domain, ipnet)
	if !ok {
		ppfmt.Errorf(pp.EmojiError, "Failed to (fully) update %s records of %q", recordType, domainDescription)
		return false
	}

	matchedIDs, unmatchedIDs := splitRecords(rs, ip)

	// whether there was already an up-to-date record
	uptodate := false
	// whether everything works
	numUnmatched := len(unmatchedIDs)

	// delete every record if ip is not valid; this means we should delete all matching records
	if !ip.IsValid() {
		uptodate = true
	}

	if !uptodate && len(matchedIDs) > 0 {
		uptodate = true
		matchedIDs = matchedIDs[1:]
	}

	if uptodate && len(matchedIDs) == 0 && len(unmatchedIDs) == 0 {
		ppfmt.Infof(pp.EmojiAlreadyDone, "The %s records of %q are already up to date", recordType, domainDescription)
		return true
	}

	if !uptodate && ip.IsValid() {
		var unhandled []string

		for i, id := range unmatchedIDs {
			if s.Handle.UpdateRecord(ctx, ppfmt, domain, ipnet, id, ip) {
				ppfmt.Noticef(pp.EmojiUpdateRecord,
					"Updated a stale %s record of %q (ID: %s)", recordType, domainDescription, id)

				uptodate = true
				numUnmatched--
				unhandled = unmatchedIDs[i+1:]

				break
			} else {
				if s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
					ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)",
						recordType, domainDescription, id)
					numUnmatched--
				}
				continue
			}
		}

		unmatchedIDs = unhandled
	}

	if !uptodate && ip.IsValid() {
		if id, ok := s.Handle.CreateRecord(ctx, ppfmt,
			domain, ipnet, ip, s.TTL, s.Proxied); ok {
			ppfmt.Noticef(pp.EmojiAddRecord, "Added a new %s record of %q (ID: %s)", recordType, domainDescription, id)
			uptodate = true
		}
	}

	for _, id := range unmatchedIDs {
		if s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
			ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a stale %s record of %q (ID: %s)", recordType, domainDescription, id)
			numUnmatched--
		}
	}

	for _, id := range matchedIDs {
		if s.Handle.DeleteRecord(ctx, ppfmt, domain, ipnet, id) {
			ppfmt.Noticef(pp.EmojiDelRecord, "Deleted a duplicate %s record of %q (ID: %s)",
				recordType, domainDescription, id)
		}
	}

	if !uptodate || numUnmatched > 0 {
		ppfmt.Errorf(pp.EmojiError, "Failed to (fully) update %s records of %q", recordType, domainDescription)
		return false
	}

	return true
}
