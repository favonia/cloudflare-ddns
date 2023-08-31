package config

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
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

// ShouldWeUse1001 quickly checks 1.1.1.1 and 1.0.0.1 and notes whether 1.0.0.1 should be used.
func (c *Config) ShouldWeUse1001(ctx context.Context, ppfmt pp.PP) bool {
	c.Use1001 = false
	if c.Provider[ipnet.IP4] == nil || !c.Provider[ipnet.IP4].ShouldWeCheck1111() {
		return true
	}

	if ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Infof(pp.EmojiEnvVars, "Checking 1.1.1.1 . . .")
		ppfmt = ppfmt.IncIndent()
	}

	if ProbeURL(ctx, "https://1.1.1.1") {
		ppfmt.Infof(pp.EmojiGood, "1.1.1.1 appears to be working")
	} else {
		ppfmt.Warningf(pp.EmojiError, "1.1.1.1 appears to be blocked or hijacked by your ISP or your router")

		if ProbeURL(ctx, "https://1.0.0.1") {
			ppfmt.Warningf(pp.EmojiGood, "1.0.0.1 appears to be working and will be used instead of 1.1.1.1")
			c.Use1001 = true
		} else {
			ppfmt.Warningf(pp.EmojiError, "1.0.0.1 is not working either---perhaps the network is down; sticking to 1.1.1.1")
		}
	}
	return true
}
