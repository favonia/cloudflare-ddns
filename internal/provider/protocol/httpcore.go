package protocol

import (
	"context"
	"io"
	"net/netip"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type httpCore struct {
	url         string
	method      string
	contentType string
	accept      string
	reader      io.Reader
	extract     func(pp.PP, []byte) (netip.Addr, bool)
}

func (d *httpCore) getIP(ctx context.Context, ppfmt pp.PP) (netip.Addr, bool) {
	var invalidIP netip.Addr

	req, err := retryablehttp.NewRequestWithContext(ctx, d.method, d.url, d.reader)
	if err != nil {
		ppfmt.Warningf(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v", d.url, err)
		return invalidIP, false
	}

	if d.contentType != "" {
		req.Header.Set("Content-Type", d.contentType)
	}

	if d.accept != "" {
		req.Header.Set("Accept", d.accept)
	}

	c := retryablehttp.NewClient()
	c.Logger = nil

	resp, err := c.Do(req)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", d.url, err)
		return invalidIP, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ppfmt.Warningf(pp.EmojiError, "Failed to read HTTP(S) response from %q: %v", d.url, err)
		return invalidIP, false
	}

	return d.extract(ppfmt, body)
}
