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
