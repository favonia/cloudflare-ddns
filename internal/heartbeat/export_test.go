package heartbeat

import "net/http"

// HealthchecksWithClient keeps endpoint behavior tests external: their generated
// PP mocks import heartbeat, so same-package tests would create an import cycle.
func HealthchecksWithClient(h Healthchecks, client *http.Client) Healthchecks {
	h.httpClient = client
	return h
}

// UptimeKumaWithClient provides the same test-only HTTP seam for Uptime Kuma.
func UptimeKumaWithClient(h UptimeKuma, client *http.Client) UptimeKuma {
	h.httpClient = client
	return h
}
