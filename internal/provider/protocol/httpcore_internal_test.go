package protocol

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

var (
	errTransportFailure = errors.New("transport failure")
	errReadFailure      = errors.New("read failure")
)

func TestHTTPCoreGetBodyWithoutRetryDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))

	h := httpCore{ //nolint:exhaustruct_v5 // GET request; no additional headers or body needed.
		ipFamily: ipnet.IP4,
		url:      "https://example.com/",
		method:   http.MethodGet,
	}
	_, _, err := h.getBodyWithoutRetry(context.Background(), server.Client())

	require.NoError(t, err)
	require.EqualValues(t, 1, requests.Load())
}

func TestHTTPCoreGetBodyWithoutRetryFollowsRedirects(t *testing.T) {
	t.Parallel()

	var sourceRequests atomic.Int64
	var targetRequests atomic.Int64
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/target" {
			targetRequests.Add(1)
			_, _ = io.WriteString(w, "redirected body")
			return
		}
		sourceRequests.Add(1)
		w.Header().Set("Location", "https://example.com/target")
		w.WriteHeader(http.StatusFound)
	}))

	h := httpCore{ //nolint:exhaustruct_v5 // GET request; no additional headers or body needed.
		ipFamily: ipnet.IP4,
		url:      "https://example.com/",
		method:   http.MethodGet,
	}
	body, finalURL, err := h.getBodyWithoutRetry(context.Background(), server.Client())

	require.NoError(t, err)
	require.Equal(t, "redirected body", string(body))
	require.Equal(t, "https://example.com/target", finalURL.String())
	require.EqualValues(t, 1, sourceRequests.Load())
	require.EqualValues(t, 1, targetRequests.Load())
}

func TestHTTPCoreGetBodyWithoutRetryAppliesAdditionalHeaders(t *testing.T) {
	t.Parallel()

	const headerName = "X-Cloudflare-Trace-Test"
	const headerValue = "present"
	receivedHeaders := make(chan http.Header, 1)
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		receivedHeaders <- req.Header.Clone()
		_, _ = io.WriteString(w, "response body")
	}))

	h := httpCore{ //nolint:exhaustruct_v5 // GET request needs only the tested headers and endpoint.
		ipFamily: ipnet.IP4,
		url:      "https://example.com/",
		method:   http.MethodGet,
		additionalHeaders: map[string]string{
			headerName: headerValue,
		},
	}
	body, _, err := h.getBodyWithoutRetry(context.Background(), server.Client())

	// Mutation caught: omitting configured headers from the request path without retries.
	require.NoError(t, err)
	require.Equal(t, []byte("response body"), body)
	require.Equal(t, headerValue, (<-receivedHeaders).Get(headerName))
}

func TestHTTPCoreGetBodyWithoutRetryReportsRequestPreparationFailure(t *testing.T) {
	t.Parallel()

	h := httpCore{ //nolint:exhaustruct_v5 // The invalid method fails before transport setup matters.
		url:    "http://example.com/",
		method: "GET\n",
	}
	body, _, err := h.getBodyWithoutRetry(context.Background(), http.DefaultClient)

	// Mutation caught: losing the request-preparation category while propagating constructor errors.
	require.Nil(t, body)
	require.ErrorContains(t, err, "failed to prepare request")
}

type trackingReadCloser struct {
	io.Reader

	closed *atomic.Bool
}

func (r trackingReadCloser) Close() error {
	r.closed.Store(true)
	return nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestHTTPCoreGetBodyWithoutRetryClosesResponseBody(t *testing.T) {
	t.Parallel()
	var closed atomic.Bool
	client := &http.Client{ //nolint:exhaustruct_v5 // Test client needs only its transport.
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{ //nolint:exhaustruct_v5 // Test response needs only status and body.
				StatusCode: http.StatusOK,
				Request:    req,
				Body: trackingReadCloser{
					Reader: strings.NewReader("response body"),
					closed: &closed,
				},
			}, nil
		}),
	}

	h := httpCore{ //nolint:exhaustruct_v5 // GET request; no additional headers or body needed.
		ipFamily: ipnet.IP4,
		url:      "http://example.com/",
		method:   http.MethodGet,
	}
	body, _, err := h.getBodyWithoutRetry(context.Background(), client)

	require.NoError(t, err)
	require.Equal(t, []byte("response body"), body)
	require.True(t, closed.Load())
}

func TestHTTPCoreGetBodyWithoutRetryReportsTransportFailure(t *testing.T) {
	t.Parallel()
	client := &http.Client{ //nolint:exhaustruct_v5 // Test client needs only its transport.
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errTransportFailure
		}),
	}

	h := httpCore{ //nolint:exhaustruct_v5 // GET request; no additional headers or body needed.
		ipFamily: ipnet.IP4,
		url:      "http://example.com/",
		method:   http.MethodGet,
	}
	body, _, err := h.getBodyWithoutRetry(context.Background(), client)

	// Mutation caught: swallowing or misclassifying a transport failure.
	require.Nil(t, body)
	require.ErrorIs(t, err, errTransportFailure)
	require.ErrorContains(t, err, "request failed")
}

func TestHTTPCoreGetBodyWithoutRetryReportsReadFailureAndClosesBody(t *testing.T) {
	t.Parallel()
	var closed atomic.Bool
	client := &http.Client{ //nolint:exhaustruct_v5 // Test client needs only its transport.
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{ //nolint:exhaustruct_v5 // Test response needs only status and body.
				StatusCode: http.StatusOK,
				Body: trackingReadCloser{
					Reader: iotest.ErrReader(errReadFailure),
					closed: &closed,
				},
			}, nil
		}),
	}

	h := httpCore{ //nolint:exhaustruct_v5 // GET request; no additional headers or body needed.
		ipFamily: ipnet.IP4,
		url:      "http://example.com/",
		method:   http.MethodGet,
	}
	body, _, err := h.getBodyWithoutRetry(context.Background(), client)

	// Mutation caught: swallowing or misclassifying a read failure, or leaking its response body.
	require.Nil(t, body)
	require.ErrorIs(t, err, errReadFailure)
	require.ErrorContains(t, err, "failed to read response")
	require.True(t, closed.Load())
}

func TestHTTPCoreGetBodyRetainsRetryableHTTPBehavior(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))

	h := httpCore{ //nolint:exhaustruct_v5 // GET request; no additional headers or body needed.
		ipFamily: ipnet.IP4,
		url:      "https://example.com/",
		method:   http.MethodGet,
	}
	client := retryablehttp.NewClient()
	client.Logger = nil
	client.HTTPClient = server.Client()
	client.RetryWaitMin = 0
	client.RetryWaitMax = 0
	_, _ = h.getBodyWithRetryableClient(context.Background(), pp.NewSilent(), client)

	require.Greater(t, requests.Load(), int64(1))
}

func TestHTTPCoreGetBodyWithRetryableClientReportsReadFailureAndClosesBody(t *testing.T) {
	t.Parallel()

	var closed atomic.Bool
	client := retryablehttp.NewClient()
	client.Logger = nil
	client.RetryMax = 0
	client.HTTPClient.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{ //nolint:exhaustruct_v5 // Test response needs only status and a controlled body.
			StatusCode: http.StatusOK,
			Body: trackingReadCloser{
				Reader: iotest.ErrReader(errReadFailure),
				closed: &closed,
			},
		}, nil
	})
	h := httpCore{ //nolint:exhaustruct_v5 // Test supplies a local retryable client directly.
		url:    "http://example.com/",
		method: http.MethodGet,
	}
	var output strings.Builder

	body, ok := h.getBodyWithRetryableClient(
		context.Background(), pp.New(&output, false, pp.Verbose), client,
	)

	// Mutation caught: treating a response-read error as success, hiding its diagnostic, or leaking its body.
	require.Nil(t, body)
	require.False(t, ok)
	require.Contains(t, output.String(), "Failed to read HTTP(S) response")
	require.Contains(t, output.String(), errReadFailure.Error())
	require.True(t, closed.Load())
}
