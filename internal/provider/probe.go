package provider

import (
	"context"
	"io"
	"net/http"
	"time"
)

// ProbedCloudflareIPs indicates ProbeCloudflareIPs has been run before.
var ProbedCloudflareIPs = false //nolint: gochecknoglobals

// UseAlternativeCloudflareIPs indicates whether 1.0.0.1 should be used instead of 1.1.1.1.
var UseAlternativeCloudflareIPs = false //nolint: gochecknoglobals

// ProbeURL quickly checks whether one can send a HEAD request to the url.
func ProbeURL(ctx context.Context, url string) bool {
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	return err == nil
}

// ProbeCloudflareIPs quickly checks which of 1.1.1.1 and 1.0.0.1
// and decide whether the alternative URL should be used.
func ProbeCloudflareIPs(ctx context.Context) {
	if !ProbedCloudflareIPs {
		ProbedCloudflareIPs = true
		UseAlternativeCloudflareIPs = !ProbeURL(ctx, "https://1.1.1.1") && ProbeURL(ctx, "https://1.0.0.1")
	}
}
