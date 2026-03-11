package api

import (
	"net/url"
	"strings"
)

func cloudflareDashboardDeeplink(segments ...string) string {
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		escaped = append(escaped, url.PathEscape(segment))
	}
	return (&url.URL{
		Scheme:   "https",
		Host:     "dash.cloudflare.com",
		Path:     "/",
		RawQuery: "to=/" + strings.Join(escaped, "/"),
	}).String()
}

func cloudflareWAFListDeeplink(accountID, listID ID) string {
	return cloudflareDashboardDeeplink(string(accountID), "configurations", "lists", string(listID))
}

func cloudflareDNSRecordsDeeplink(accountID, zoneID ID) string {
	return cloudflareDashboardDeeplink(string(accountID), string(zoneID), "dns", "records")
}
