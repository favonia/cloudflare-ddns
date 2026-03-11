package api

import "testing"

func TestCloudflareWAFListDeeplink(t *testing.T) {
	t.Parallel()

	got := cloudflareWAFListDeeplink("account 123", "list/456")
	want := "https://dash.cloudflare.com/?to=/account%20123/configurations/lists/list%2F456"
	if got != want {
		t.Fatalf("cloudflareWAFListDeeplink() = %q, want %q", got, want)
	}
}

func TestCloudflareDNSRecordsDeeplink(t *testing.T) {
	t.Parallel()

	got := cloudflareDNSRecordsDeeplink("account+123", "zone/456")
	want := "https://dash.cloudflare.com/?to=/account+123/zone%2F456/dns/records"
	if got != want {
		t.Fatalf("cloudflareDNSRecordsDeeplink() = %q, want %q", got, want)
	}
}
