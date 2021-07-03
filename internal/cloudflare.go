package ddns

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/cloudflare/cloudflare-go"
)

type handle struct {
	cfHandle  *cloudflare.API
	ip4Set    bool
	ip4Cached net.IP
	ip6Set    bool
	ip6Cached net.IP
}

func newHandleWithKey(key, email string) (*handle, error) {
	cfHandle, err := cloudflare.New(key, email)
	if err != nil {
		return nil, fmt.Errorf("😡 The token-based CloudFlare authentication failed: %w", err)
	}
	return &handle{cfHandle: cfHandle}, nil
}

func newHandleWithToken(token string) (*handle, error) {
	cfHandle, err := cloudflare.NewWithAPIToken(token)
	if err != nil {
		return nil, fmt.Errorf("😡 The key-based CloudFlare authentication failed: %w", err)
	}
	return &handle{cfHandle: cfHandle}, nil
}

func (h *handle) zoneDetails(ctx context.Context, zoneID string) (cloudflare.Zone, error) {
	zone, err := h.cfHandle.ZoneDetails(ctx, zoneID)
	if err != nil {
		return cloudflare.Zone{}, fmt.Errorf("😡 Could not retrieve the information of the zone (ID: %s): %w", zoneID, err)
	}
	return zone, nil
}

func (h *handle) findZone(ctx context.Context, fqdnString string) (zone cloudflare.Zone, err error) {
	fqdn := []byte(fqdnString)

	// try the whole domain as the zone
	zones, err := h.cfHandle.ListZones(ctx, fqdnString)
	if err == nil && len(zones) > 0 {
		return zones[0], nil
	}

	// search for the closetest zone
	for i, b := range fqdn {
		if b != '.' {
			continue
		}
		zoneName := string(fqdn[i+1:])
		zones, err = h.cfHandle.ListZones(ctx, zoneName)
		if err == nil && len(zones) > 0 {
			return zones[0], nil
		}
	}
	return cloudflare.Zone{}, fmt.Errorf("🤔 Could not find a zone for the domain %s.", fqdnString)
}

func (h *handle) updateRecords(ctx context.Context, zone cloudflare.Zone, fqdn string, recordType string, ip net.IP, ttl int, proxied bool) (net.IP, error) {
	query := cloudflare.DNSRecord{Name: fqdn, Type: recordType}
	rs, err := h.cfHandle.DNSRecords(ctx, zone.ID, query)
	if err != nil {
		return nil, err
	}
	updated := false

	// delete every record if the ip is `nil`
	if ip == nil {
		updated = true
	}

	for _, r := range rs {
		if r.Name != fqdn {
			return nil, fmt.Errorf("🤯 Unexpected DNS record when handling the domain %s: %+v", fqdn, r)
		}
		if r.Type != recordType {
			return nil, fmt.Errorf("🤯 Unexpected DNS record when handling %s records: %+v", recordType, r)
		}
	}

	/*
		Find the entries that match, if any. If there is already a record that is up-to-date,
		then we will delete the first stale record instead of updating it.
	*/
	for _, r := range rs {
		if ip.Equal(net.ParseIP(r.Content)) {
			log.Printf("👌 Found an up-to-date record for the domain %s.", fqdn)
			updated = true
			break
		}
	}

	payload := cloudflare.DNSRecord{
		Name:    fqdn,
		Type:    recordType,
		Content: ip.String(),
		TTL:     ttl,
		Proxied: &proxied,
	}

	num_matched := 0
	for _, r := range rs {
		if ip.Equal(net.ParseIP(r.Content)) {
			updated = true
			num_matched++
			if num_matched > 1 {
				log.Printf("👻 Removing a duplicate %s record (ID: %s) . . .", recordType, r.ID)
				err := h.cfHandle.DeleteDNSRecord(ctx, zone.ID, r.ID)
				if err != nil {
					log.Printf("😡 Could not remove the record: %v", err)
				}
			}
		} else {
			if updated {
				log.Printf("🧟 Deleting a stale %s record (ID: %s) that was pointing to %s . . .", recordType, r.ID, r.Content)
				err := h.cfHandle.DeleteDNSRecord(ctx, zone.ID, r.ID)
				if err != nil {
					log.Printf("😡 Could not delete the record: %v", err)
				}
			} else {
				log.Printf("✍️ Updating a stale %s record (ID: %s) from %s to %v . . .", recordType, r.ID, r.Content, ip)
				h.cfHandle.UpdateDNSRecord(ctx, zone.ID, r.ID, payload)
				if err != nil {
					log.Printf("😡 Could not update the record: %v", err)
					log.Printf("🧟 Deleting the record instead . . .")
					err := h.cfHandle.DeleteDNSRecord(ctx, zone.ID, r.ID)
					if err != nil {
						log.Printf("😡 Could not delete the record, either: %v", err)
					}
				} else {
					updated = true
					num_matched++
				}
			}
		}
	}

	// The remaining case: there aren't any records to begin with!
	if !updated {
		log.Printf("👶 Adding a new %s record: %+v", recordType, payload)
		_, err := h.cfHandle.CreateDNSRecord(ctx, zone.ID, payload)
		if err != nil {
			log.Printf("😡 Could not add the record: %v", err)
		} else {
			updated = true
			num_matched++
		}
	}

	if !updated {
		return nil, fmt.Errorf("😡 Failed to update %s records for the domain %s.", recordType, fqdn)
	} else {
		return ip, nil
	}
}

type DNSSetting struct {
	FQDN       string
	IP4Managed bool
	IP4        net.IP
	IP6Managed bool
	IP6        net.IP
	TTL        int
	Proxied    bool
}

func (h *handle) UpdateDNSRecords(ctx context.Context, s *DNSSetting) error {
	checkingIP4 := s.IP4Managed && !(h.ip4Set && h.ip4Cached.Equal(s.IP4))
	checkingIP6 := s.IP6Managed && !(h.ip6Set && h.ip6Cached.Equal(s.IP6))
	if !checkingIP4 && !checkingIP6 {
		log.Printf("🤷 Nothing to do; skipping the updating.")
		return nil
	}

	zone, err := h.findZone(ctx, s.FQDN)
	if err != nil {
		return err
	}
	log.Printf("🧐 Found the zone at %s for the domain %s.", zone.Name, s.FQDN)

	var (
		err4, err6 error
		ip         net.IP
	)

	if checkingIP4 {
		ip, err4 = h.updateRecords(ctx, zone, s.FQDN, "A", s.IP4.To4(), s.TTL, s.Proxied)
		if err != nil {
			h.ip4Set = false
		} else {
			h.ip4Set = true
			h.ip4Cached = ip
		}
	}

	if checkingIP6 {
		ip, err6 = h.updateRecords(ctx, zone, s.FQDN, "AAAA", s.IP6.To16(), s.TTL, s.Proxied)
		if err != nil {
			h.ip6Set = false
		} else {
			h.ip6Set = true
			h.ip6Cached = ip
		}
	}

	if err4 != nil || err6 != nil {
		return fmt.Errorf("😡 Failed to update records for the domain %s.", s.FQDN)
	} else {
		return nil
	}
}
