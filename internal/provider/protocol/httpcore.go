package protocol

import (
	"context"
	"io"
	"net/netip"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// maxReadLength is the maximum number of bytes read from an HTTP response.
const maxReadLength int64 = 102400

type httpCore struct {
	url               string
	method            string
	additionalHeaders map[string]string
	requestBody       io.Reader
	extract           func(pp.PP, []byte) (netip.Addr, bool)
}

func (h httpCore) getIP(ctx context.Context, ppfmt pp.PP) (netip.Addr, bool) {
	var invalidIP netip.Addr

	req, err := retryablehttp.NewRequestWithContext(ctx, h.method, h.url, h.requestBody)
	if err != nil {
		ppfmt.Warningf(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v", h.url, err)
		return invalidIP, false
	}

	for header, value := range h.additionalHeaders {
		req.Header.Set(header, value)
	}

	c := retryablehttp.NewClient()
	c.Logger = nil

	resp, err := c.Do(req)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", h.url, err)
		return invalidIP, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReadLength))
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to read HTTP(S) response from %q: %v", h.url, err)
		return invalidIP, false
	}

	return h.extract(ppfmt, body)
}
