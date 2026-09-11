// This file provides HTTP client injection for external heartbeat tests.
// These tests use the shared mocks package, which imports heartbeat;
// placing them in package heartbeat would create an import cycle.

package heartbeat

import "net/http"

// WithHTTPClient returns a copy that uses client for subsequent pings.
// A nil client selects http.DefaultClient; the original heartbeat is unchanged.
func (h Healthchecks) WithHTTPClient(client *http.Client) Healthchecks {
	h.httpClient = client
	return h
}

// WithHTTPClient returns a copy that uses client for subsequent pings.
// A nil client selects http.DefaultClient; the original heartbeat is unchanged.
func (h UptimeKuma) WithHTTPClient(client *http.Client) UptimeKuma {
	h.httpClient = client
	return h
}
