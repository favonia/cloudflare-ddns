package ddns

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/cloudflare/cloudflare-go"
)

type handle struct {
	api        *cloudflare.API
	currentIP4 *net.IP
	currentIP6 *net.IP
}

func NewAPI(token string) (*handle, error) {
	api, err := cloudflare.NewWithAPIToken(token)
	if err != nil {
		return nil, err
	}
	return &handle{
		api:        api,
		currentIP4: nil,
		currentIP6: nil,
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
			return nil, fmt.Errorf("🙀 Unexpected DNS record %+v when searching for the domain %s", r, fqdn)
		}
		if r.Type != recordType {
			return nil, fmt.Errorf("🙀 Unexpected DNS record %+v when searching for records of type %s", r, recordType)
		}
	}
	for _, r := range rs {
		if ip.Equal(net.ParseIP(r.Content)) {
			log.Printf("✅ Record was already up-to-date (%s).", ip.String())
			updated = true
			continue
		}
	}

	payload := cloudflare.DNSRecord{
		Name:    fqdn,
		Type:    recordType,
		Content: ip.String(),
		TTL:     ttl,
		Proxied: &proxied,
	}

	for _, r := range rs {
		if !ip.Equal(net.ParseIP(r.Content)) {
			if updated {
				log.Printf("🗑️ Deleting stale record pointing to %s", r.Content)
				h.api.DeleteDNSRecord(ctx, zone.ID, r.ID)
			} else {
				log.Printf("📡 Updating record %+v", payload)
				h.api.UpdateDNSRecord(ctx, zone.ID, r.ID, payload)
				updated = true
			}
			updated = true
			continue
		}
	}
	if !updated {
		log.Printf("➕ Adding new record %+v", payload)
		h.api.CreateDNSRecord(ctx, zone.ID, payload)
		updated = true
	}
	return ip, nil
}

type DNSSetting struct {
	FQDN    string
	IP4     *net.IP
	IP6     *net.IP
	TTL     int
	Proxied bool
}

func (h *handle) UpdateDNSRecords(ctx context.Context, s DNSSetting) error {
	if s.IP4 != nil && h.currentIP4 != nil && h.currentIP4.Equal(*s.IP4) {
		s.IP4 = nil // as if the policy is "disabled"
	}
	if s.IP6 != nil && h.currentIP6 != nil && h.currentIP6.Equal(*s.IP6) {
		s.IP6 = nil // as if the policy is "disabled"
	}
	if s.IP4 == nil && s.IP6 == nil {
		log.Printf("🤷 Nothing to be done; skipping the updating.")
		return nil
	}

	zone, err := h.findZone(ctx, s.FQDN)
	if err != nil {
		return err
	}
	log.Printf("🔍 Found the zone %s for the domain %s.", zone.Name, s.FQDN)

	if s.IP4 != nil {
		ip, err := h.updateRecords(ctx, zone, s.FQDN, "A", s.IP4.To4(), s.TTL, s.Proxied)
		if err != nil {
			h.currentIP4 = nil
			return err
		}
		h.currentIP4 = &ip
	}

	if s.IP6 != nil {
		ip, err := h.updateRecords(ctx, zone, s.FQDN, "AAAA", s.IP6.To16(), s.TTL, s.Proxied)
		if err != nil {
			h.currentIP6 = nil
			return err
		}
		h.currentIP6 = &ip
	}

	return nil
}
