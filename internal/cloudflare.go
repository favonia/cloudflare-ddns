package ddns

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/cloudflare/cloudflare-go"
)

type handle struct {
	api       *cloudflare.API
	ip4Set    bool
	ip4Cached net.IP
	ip6Set    bool
	ip6Cached net.IP
}

func NewAPI(token string) (*handle, error) {
	api, err := cloudflare.NewWithAPIToken(token)
	if err != nil {
		return nil, err
	}
	return &handle{
		api:       api,
		ip4Set:    false,
		ip4Cached: nil,
		ip6Set:    false,
		ip6Cached: nil,
	}, nil
}

func (h *handle) findZone(ctx context.Context, fqdnString string) (zone *cloudflare.Zone, err error) {
	fqdn := []byte(fqdnString)

	// try the whole domain
	zones, err := h.api.ListZones(ctx, fqdnString)
	if err == nil && len(zones) > 0 {
		return &zones[0], nil
	}

	// search for the closet subdomain
	for i, b := range fqdn {
		if b != '.' {
			continue
		}
		zoneName := string(fqdn[i+1:])
		zones, err = h.api.ListZones(ctx, zoneName)
		if err == nil && len(zones) > 0 {
			return &zones[0], nil
		}
	}
	return nil, fmt.Errorf("🤔 Couldn't find the zone for the domain: %s.", fqdnString)
}

func (h *handle) updateRecords(ctx context.Context, zone *cloudflare.Zone, fqdn string, recordType string, ip net.IP, ttl int, proxied bool) (net.IP, error) {
	query := cloudflare.DNSRecord{Name: fqdn, Type: recordType}
	rs, err := h.api.DNSRecords(ctx, zone.ID, query)
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
			return nil, fmt.Errorf("🤯 Unexpected DNS record %+v when searching for the domain %s", r, fqdn)
		}
		if r.Type != recordType {
			return nil, fmt.Errorf("🤯 Unexpected DNS record %+v when searching for records of type %s", r, recordType)
		}
	}

	// find the entries that match
	for _, r := range rs {
		if ip.Equal(net.ParseIP(r.Content)) {
			log.Printf("✅ Record was already up-to-date (%s).", ip.String())
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

	found_matched := 0
	for _, r := range rs {
		if ip.Equal(net.ParseIP(r.Content)) {
			found_matched++
			if found_matched > 1 {
				log.Printf("🧹 Removing a duplicate record: %+v", r)
				h.api.DeleteDNSRecord(ctx, zone.ID, r.ID)
			}
		} else {
			if updated {
				log.Printf("🗑️ Deleting a stale record pointing to %s", r.Content)
				h.api.DeleteDNSRecord(ctx, zone.ID, r.ID)
			} else {
				log.Printf("📡 Updating a record from %s to %v", r.Content, ip)
				h.api.UpdateDNSRecord(ctx, zone.ID, r.ID, payload)
				updated = true
			}
		}
	}
	if !updated {
		log.Printf("➕ Adding a new record %+v", payload)
		h.api.CreateDNSRecord(ctx, zone.ID, payload)
		updated = true
	}
	return ip, nil
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
	checkingIP4 := s.IP4Managed
	if checkingIP4 && h.ip4Set && h.ip4Cached.Equal(s.IP4) {
		checkingIP4 = false // as if the policy is "unmanaged"
	}
	checkingIP6 := s.IP6Managed
	if checkingIP6 && h.ip6Set && h.ip6Cached.Equal(s.IP6) {
		checkingIP6 = false // as if the policy is "unmanaged"
	}
	if !checkingIP4 && !checkingIP6 {
		log.Printf("🤷 Nothing to do; skipping the updating.")
		return nil
	}

	zone, err := h.findZone(ctx, s.FQDN)
	if err != nil {
		return err
	}
	log.Printf("🔍 Found the zone %s for the domain %s.", zone.Name, s.FQDN)

	if checkingIP4 {
		ip, err := h.updateRecords(ctx, zone, s.FQDN, "A", s.IP4.To4(), s.TTL, s.Proxied)
		if err != nil {
			h.ip4Set = false
			return err
		} else {
			h.ip4Set = true
			h.ip4Cached = ip
		}
	}

	if checkingIP6 {
		ip, err := h.updateRecords(ctx, zone, s.FQDN, "AAAA", s.IP6.To16(), s.TTL, s.Proxied)
		if err != nil {
			h.ip6Set = false
			return err
		} else {
			h.ip6Set = true
			h.ip6Cached = ip
		}
	}

	return nil
}
