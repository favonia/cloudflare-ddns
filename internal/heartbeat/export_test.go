package heartbeat

import "net/http"

// SetHTTPClient overrides the client for subsequent pings in tests; nil restores
// http.DefaultClient. Call it before using the heartbeat concurrently.
// External tests need this hook because their generated PP mocks import heartbeat.
func (h *Healthchecks) SetHTTPClient(client *http.Client) {
	h.httpClient = client
}

// SetHTTPClient overrides the client for subsequent pings in tests; nil restores
// http.DefaultClient. Call it before using the heartbeat concurrently.
func (h *UptimeKuma) SetHTTPClient(client *http.Client) {
	h.httpClient = client
}
