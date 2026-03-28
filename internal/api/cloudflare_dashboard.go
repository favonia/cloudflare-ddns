package api

import (
	"net/url"
	"strings"
)

// The published dashboard-route anchor snapshot below was adopted on
// 2026-03-22 and is watched by the Cloudflare dashboard deeplink anchors case
// in scripts/github-actions/cloudflare-doc-watch/cases.go.
//
// Cloudflare dashboard deeplink provenance:
//   - DNS records uses the documented "Records" route from
//     src/content/dash-routes/core.json: /:account/:zone/dns/records.
//   - WAF lists uses the documented account-level "Configurations" route
//     prefix from src/content/dash-routes/core.json: /:account/configurations,
//     plus the unofficial /lists/:list-id suffix observed to work in practice.
func cloudflareDashboardDeeplink(segments ...string) string {
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		// The "to" query parameter is itself a dashboard path. Escape each segment
		// first so reserved characters inside IDs stay data, then let query.Encode
		// escape the full path for the outer URL. This yields the intentional
		// double-encoding seen for characters such as "/" (%2F -> %252F).
		escaped = append(escaped, url.PathEscape(segment))
	}
	query := url.Values{}
	query.Set("to", "/"+strings.Join(escaped, "/"))
	//nolint:exhaustruct // url.URL is intentionally populated with only the fields used here.
	return (&url.URL{
		Scheme:   "https",
		Host:     "dash.cloudflare.com",
		Path:     "/",
		RawQuery: query.Encode(),
	}).String()
}

func cloudflareWAFListDeeplink(accountID, listID ID) string {
	return cloudflareDashboardDeeplink(string(accountID), "configurations", "lists", string(listID))
}

func cloudflareDNSRecordsDeeplink(accountID, zoneID ID) string {
	return cloudflareDashboardDeeplink(string(accountID), string(zoneID), "dns", "records")
}
