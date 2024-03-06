package config

import (
	"context"
	"net/http"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ProbeURL quickly checks whether one can send a HEAD request to the url.
func ProbeURL(ctx context.Context, url string) bool {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	err = resp.Body.Close()
	return err == nil
}

// ShouldWeUse1001 quickly checks 1.1.1.1 and 1.0.0.1 and notes whether 1.0.0.1 should be used.
//
// Note that the return value is about whether the detection is successfully done, not that
// whether we should use 1.0.0.1. The function will update the field [Config.Use1001] directly.
func (c *Config) ShouldWeUse1001(ctx context.Context, ppfmt pp.PP) bool {
	c.Use1001 = false
	if c.Provider[ipnet.IP4] == nil || !c.Provider[ipnet.IP4].ShouldWeCheck1111() {
		return true
	}

	if ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Infof(pp.EmojiEnvVars, "Probing 1.1.1.1 and 1.0.0.1 . . .")
		ppfmt = ppfmt.IncIndent()
	}

	if ProbeURL(ctx, "https://1.1.1.1") {
		ppfmt.Infof(pp.EmojiGood, "1.1.1.1 is working. Great!")
	} else {
		if ProbeURL(ctx, "https://1.0.0.1") {
			ppfmt.Warningf(pp.EmojiError, "1.1.1.1 is not working, but 1.0.0.1 is; using 1.0.0.1")
			ppfmt.Infof(pp.EmojiHint, "1.1.1.1 is probably blocked or hijacked by your router or ISP")
			c.Use1001 = true
		} else {
			ppfmt.Warningf(pp.EmojiError, "Both 1.1.1.1 and 1.0.0.1 are not working; sticking to 1.1.1.1")
			ppfmt.Infof(pp.EmojiHint, "The network might be temporarily down, or has not been set up yet")
			ppfmt.Infof(pp.EmojiHint, "If you start this tool during booting, make sure the network is already up")
		}
	}
	return true
}
