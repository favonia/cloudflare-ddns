package protocol

import (
	"context"
	"io"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// defaultMaxReadLength is the maximum number of bytes read from an HTTP response
// when no per-instance limit is set.
const defaultMaxReadLength int64 = 102400

type httpCore struct {
	ipFamily          ipnet.Family
	url               string
	method            string
	additionalHeaders map[string]string
	requestBody       io.Reader
	maxReadLength     int64 // 0 means use defaultMaxReadLength
}

func (h httpCore) getBody(ctx context.Context, ppfmt pp.PP) ([]byte, bool) {
	req, err := retryablehttp.NewRequestWithContext(ctx, h.method, h.url, h.requestBody)
	if err != nil {
		ppfmt.Noticef(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v", h.url, err)
		return nil, false
	}

	for header, value := range h.additionalHeaders {
		req.Header.Set(header, value)
	}

	c := SharedRetryableSplitClient(h.ipFamily)

	resp, err := c.Do(req)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", h.url, err)
		return nil, false
	}
	defer resp.Body.Close()

	limit := h.maxReadLength
	if limit <= 0 {
		limit = defaultMaxReadLength
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to read HTTP(S) response from %q: %v", h.url, err)
		return nil, false
	}

	return body, true
}
