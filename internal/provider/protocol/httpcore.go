package protocol

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

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
	return h.getBodyWithRetryableClient(ctx, ppfmt, SharedRetryableSplitClient(h.ipFamily))
}

func (h httpCore) getBodyWithRetryableClient(
	ctx context.Context,
	ppfmt pp.PP,
	client *retryablehttp.Client,
) ([]byte, bool) {
	displayURL := pp.QuoteIfUnsafeInSentence(h.url)
	req, err := retryablehttp.NewRequestWithContext(ctx, h.method, h.url, h.requestBody)
	if err != nil {
		ppfmt.Noticef(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %s: %v", displayURL, err)
		return nil, false
	}

	for header, value := range h.additionalHeaders {
		req.Header.Set(header, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to send HTTP(S) request to %s: %v", displayURL, err)
		return nil, false
	}
	defer resp.Body.Close()

	body, err := h.readBody(resp.Body)
	if err != nil {
		ppfmt.Noticef(pp.EmojiError, "Failed to read HTTP(S) response from %s: %v", displayURL, err)
		return nil, false
	}

	return body, true
}

// getBodyWithoutRetry currently serves only attemptCloudflareTrace, whose
// hedging coordinator schedules alternative attempts. It uses the supplied
// HTTP client without adding retries or changing client policy; redirects
// and transport-level retries can still send additional requests. The caller
// must bound ctx and supply an appropriate client.
//
// On success it returns the size-limited body and final request URL after any
// redirects, with the response body closed. On failure both returned values are
// nil. It does not reject HTTP status codes: Cloudflare Trace validates the body
// and its h field against the final URL. Other callers must define their own
// status and body validation before reusing this helper.
func (h httpCore) getBodyWithoutRetry(ctx context.Context, client *http.Client) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, h.method, h.url, h.requestBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare request: %w", err)
	}
	for header, value := range h.additionalHeaders {
		req.Header.Set(header, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := h.readBody(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}
	return body, resp.Request.URL, nil
}

func (h httpCore) readBody(reader io.Reader) ([]byte, error) {
	limit := h.maxReadLength
	if limit <= 0 {
		limit = defaultMaxReadLength
	}
	// The caller adds transport-specific context to any response-read failure.
	return io.ReadAll(io.LimitReader(reader, limit)) //nolint:wrapcheck // Preserve caller-specific error context.
}
