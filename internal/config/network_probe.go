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

// ShouldWeUse1001Now quickly checks 1.1.1.1 and 1.0.0.1 and notes whether 1.0.0.1 should be used.
//
// [Config.ShouldWeUse1001] remembers the results.
func (c *Config) ShouldWeUse1001Now(ctx context.Context, ppfmt pp.PP) bool {
	if c.ShouldWeUse1001 != nil {
		return *c.ShouldWeUse1001
	}
	if c.Provider[ipnet.IP4] == nil || !c.Provider[ipnet.IP4].ShouldWeCheck1111() {
		return false // any answer would work
	}

	if ppfmt.Verbosity() >= pp.Info {
		ppfmt.Infof(pp.EmojiEnvVars, "Probing 1.1.1.1 and 1.0.0.1 . . .")
		ppfmt = ppfmt.Indent()
	}

	if ProbeURL(ctx, "https://1.1.1.1") {
		ppfmt.Infof(pp.EmojiGood, "1.1.1.1 is working. Great!")

		res := false
		c.ShouldWeUse1001 = &res
		return false
	} else {
		if ProbeURL(ctx, "https://1.0.0.1") {
			ppfmt.Noticef(pp.EmojiError, "1.1.1.1 is not working, but 1.0.0.1 is; using 1.0.0.1")
			ppfmt.Infof(pp.EmojiHint, "1.1.1.1 is probably blocked or hijacked by your router or ISP")

			res := true
			c.ShouldWeUse1001 = &res
			return true
		} else {
			ppfmt.Noticef(pp.EmojiError, "Both 1.1.1.1 and 1.0.0.1 are not working; sticking to 1.1.1.1 now")
			ppfmt.Infof(pp.EmojiHint, "The network might be temporarily down; will redo probing later")
			return false
		}
	}
}
