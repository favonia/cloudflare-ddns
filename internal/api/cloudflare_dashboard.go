package api

import (
	"net/url"
	"strings"
)

// Cloudflare dashboard deeplink provenance:
//   - DNS records uses a documented "to" target published in Cloudflare's
//     dash-routes/core.json: /:account/:zone/dns/records.
//   - WAF lists currently uses an undocumented dashboard path observed to work in
//     practice.
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
