package heartbeat

import "net/http"

// HealthchecksWithHTTPClient keeps endpoint behavior tests external: their generated
// PP mocks import heartbeat, so same-package tests would create an import cycle.
func HealthchecksWithHTTPClient(h Healthchecks, client *http.Client) Healthchecks {
	h.httpClient = client
	return h
}

// UptimeKumaWithHTTPClient provides the same test-only HTTP seam for Uptime Kuma.
func UptimeKumaWithHTTPClient(h UptimeKuma, client *http.Client) UptimeKuma {
	h.httpClient = client
	return h
}
