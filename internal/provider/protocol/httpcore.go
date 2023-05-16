package protocol

import (
	"context"
	"io"
	"net/netip"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type httpCore struct {
	ipNet       ipnet.Type
	url         string
	method      string
	contentType string
	accept      string
	reader      io.Reader
	extract     func(pp.PP, []byte) (netip.Addr, bool)
}

func (h *httpCore) getIP(ctx context.Context, ppfmt pp.PP) (netip.Addr, bool) {
	var invalidIP netip.Addr

	req, err := retryablehttp.NewRequestWithContext(ctx, h.method, h.url, h.reader)
	if err != nil {
		ppfmt.Warningf(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v", h.url, err)
		return invalidIP, false
	}

	if !ipnet.ForceResolveRetryableRequest(ctx, h.ipNet, req) {
		ppfmt.Warningf(pp.EmojiError,
			"Failed to force resolve the host of %q as an %s address",
			h.url, h.ipNet.Describe())
		return invalidIP, false
	}

	if h.contentType != "" {
		req.Header.Set("Content-Type", h.contentType)
	}

	if h.accept != "" {
		req.Header.Set("Accept", h.accept)
	}

	c := retryablehttp.NewClient()
	c.Logger = nil

	resp, err := c.Do(req)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", h.url, err)
		return invalidIP, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to read HTTP(S) response from %q: %v", h.url, err)
		return invalidIP, false
	}

	return h.extract(ppfmt, body)
}
