package api

import (
	"net/url"
	"testing"
)

func TestCloudflareWAFListDeeplink(t *testing.T) {
	t.Parallel()

	got := cloudflareWAFListDeeplink("account 123", "list/456")
	want := "https://dash.cloudflare.com/?to=%2Faccount%2520123%2Fconfigurations%2Flists%2Flist%252F456"
	if got != want {
		t.Fatalf("cloudflareWAFListDeeplink() = %q, want %q", got, want)
	}
}

func TestCloudflareDNSRecordsDeeplink(t *testing.T) {
	t.Parallel()

	got := cloudflareDNSRecordsDeeplink("account+123", "zone/456")
	want := "https://dash.cloudflare.com/?to=%2Faccount%2B123%2Fzone%252F456%2Fdns%2Frecords"
	if got != want {
		t.Fatalf("cloudflareDNSRecordsDeeplink() = %q, want %q", got, want)
	}
}

func TestCloudflareDashboardDeeplinkRoundTrip(t *testing.T) {
	t.Parallel()

	got := cloudflareDNSRecordsDeeplink("account+123", "zone/456")
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse(%q) failed: %v", got, err)
	}

	// "to" is a query parameter whose value is itself a dashboard path.
	// A slash inside one segment must therefore survive one query decode as
	// escaped path data rather than turning into a path separator.
	want := "/account+123/zone%2F456/dns/records"
	if decoded := parsed.Query().Get("to"); decoded != want {
		t.Fatalf("decoded to query = %q, want %q", decoded, want)
	}
}
