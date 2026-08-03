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

func TestHTTPCoreGetBodyOnceDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	h := httpCore{ //nolint:exhaustruct // GET request; no additional headers or body needed.
		ipFamily: ipnet.IP4,
		url:      server.URL,
		method:   http.MethodGet,
	}
	_, err := h.getBodyOnce(context.Background())

	require.NoError(t, err)
	require.EqualValues(t, 1, requests.Load())
}

func TestHTTPCoreGetBodyOnceDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	var sourceRequests atomic.Int64
	var targetRequests atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetRequests.Add(1)
	}))
	t.Cleanup(target.Close)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sourceRequests.Add(1)
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(source.Close)

	h := httpCore{ //nolint:exhaustruct // GET request; no additional headers or body needed.
		ipFamily: ipnet.IP4,
		url:      source.URL,
		method:   http.MethodGet,
	}
	_, err := h.getBodyOnce(context.Background())

	require.NoError(t, err)
	require.EqualValues(t, 1, sourceRequests.Load())
	require.EqualValues(t, 0, targetRequests.Load())
}

func TestHTTPCoreGetBodyOnceAppliesAdditionalHeaders(t *testing.T) {
	t.Parallel()

	const headerName = "X-Cloudflare-Trace-Test"
	const headerValue = "present"
	receivedHeaders := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		receivedHeaders <- req.Header.Clone()
		_, _ = io.WriteString(w, "response body")
	}))
	t.Cleanup(server.Close)

	h := httpCore{ //nolint:exhaustruct // GET request needs only the tested headers and endpoint.
		ipFamily: ipnet.IP4,
		url:      server.URL,
		method:   http.MethodGet,
		additionalHeaders: map[string]string{
			headerName: headerValue,
		},
	}
	body, err := h.getBodyOnce(context.Background())

	// Mutation caught: omitting configured headers from the one-shot request path.
	require.NoError(t, err)
	require.Equal(t, []byte("response body"), body)
	require.Equal(t, headerValue, (<-receivedHeaders).Get(headerName))
}

func TestHTTPCoreGetBodyOnceReportsRequestPreparationFailure(t *testing.T) {
	t.Parallel()

	h := httpCore{ //nolint:exhaustruct // The invalid method fails before transport setup matters.
		url:    "http://example.com/",
		method: "GET\n",
	}
	body, err := h.getBodyOnce(context.Background())

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

func TestHTTPCoreGetBodyOnceClosesResponseBody(t *testing.T) { //nolint:paralleltest // Mutates sharedSplitClient.
	const testFamily ipnet.Family = 99
	var closed atomic.Bool
	sharedSplitClient[testFamily] = &http.Client{ //nolint:exhaustruct // Test client needs only its transport.
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{ //nolint:exhaustruct // Test response needs only status and body.
				StatusCode: http.StatusOK,
				Body: trackingReadCloser{
					Reader: strings.NewReader("response body"),
					closed: &closed,
				},
			}, nil
		}),
	}
	t.Cleanup(func() { delete(sharedSplitClient, testFamily) })

	h := httpCore{ //nolint:exhaustruct // GET request; no additional headers or body needed.
		ipFamily: testFamily,
		url:      "http://example.com/",
		method:   http.MethodGet,
	}
	body, err := h.getBodyOnce(context.Background())

	require.NoError(t, err)
	require.Equal(t, []byte("response body"), body)
	require.True(t, closed.Load())
}

func TestHTTPCoreGetBodyOnceReportsTransportFailure(t *testing.T) { //nolint:paralleltest // Mutates sharedSplitClient.
	const testFamily ipnet.Family = 100
	sharedSplitClient[testFamily] = &http.Client{ //nolint:exhaustruct // Test client needs only its transport.
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errTransportFailure
		}),
	}
	t.Cleanup(func() { delete(sharedSplitClient, testFamily) })

	h := httpCore{ //nolint:exhaustruct // GET request; no additional headers or body needed.
		ipFamily: testFamily,
		url:      "http://example.com/",
		method:   http.MethodGet,
	}
	body, err := h.getBodyOnce(context.Background())

	// Mutation caught: swallowing or misclassifying a one-shot transport failure.
	require.Nil(t, body)
	require.ErrorIs(t, err, errTransportFailure)
	require.ErrorContains(t, err, "request failed")
}

func TestHTTPCoreGetBodyOnceReportsReadFailureAndClosesBody(t *testing.T) { //nolint:paralleltest // Mutates sharedSplitClient.
	const testFamily ipnet.Family = 101
	var closed atomic.Bool
	sharedSplitClient[testFamily] = &http.Client{ //nolint:exhaustruct // Test client needs only its transport.
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{ //nolint:exhaustruct // Test response needs only status and body.
				StatusCode: http.StatusOK,
				Body: trackingReadCloser{
					Reader: iotest.ErrReader(errReadFailure),
					closed: &closed,
				},
			}, nil
		}),
	}
	t.Cleanup(func() { delete(sharedSplitClient, testFamily) })

	h := httpCore{ //nolint:exhaustruct // GET request; no additional headers or body needed.
		ipFamily: testFamily,
		url:      "http://example.com/",
		method:   http.MethodGet,
	}
	body, err := h.getBodyOnce(context.Background())

	// Mutation caught: swallowing or misclassifying a read failure, or leaking its response body.
	require.Nil(t, body)
	require.ErrorIs(t, err, errReadFailure)
	require.ErrorContains(t, err, "failed to read response")
	require.True(t, closed.Load())
}

func TestHTTPCoreGetBodyRetainsRetryableHTTPBehavior(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	h := httpCore{ //nolint:exhaustruct // GET request; no additional headers or body needed.
		ipFamily: ipnet.IP4,
		url:      server.URL,
		method:   http.MethodGet,
	}
	client := SharedRetryableSplitClient(ipnet.IP4)
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
		return &http.Response{ //nolint:exhaustruct // Test response needs only status and a controlled body.
			StatusCode: http.StatusOK,
			Body: trackingReadCloser{
				Reader: iotest.ErrReader(errReadFailure),
				closed: &closed,
			},
		}, nil
	})
	h := httpCore{ //nolint:exhaustruct // Test supplies a local retryable client directly.
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
