package config

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

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

// ProbeCloudflareIPs quickly checks 1.1.1.1 and 1.0.0.1
// and return whether the alternative URL should be used.
func ProbeCloudflareIPs(ctx context.Context, ppfmt pp.PP) bool {
	good1111 := ProbeURL(ctx, "https://1.1.1.1")
	good1001 := ProbeURL(ctx, "https://1.0.0.1")

	if !good1111 && good1001 {
		ppfmt.Warningf(pp.EmojiError, "1.1.1.1 might have been blocked or intercepted by your ISP or your router")
		ppfmt.Warningf(pp.EmojiError, "1.0.0.1 seems to work and will be used instead")
		return true
	}

	return false
}
