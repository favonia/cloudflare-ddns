package protocol

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func newTraceAttemptServer(
	t *testing.T, ipFamily ipnet.Family, response func(*http.Request) string,
) *httptest.Server {
	t.Helper()
	network := "tcp4"
	address := "127.0.0.1:0"
	if ipFamily == ipnet.IP6 {
		network = "tcp6"
		address = "[::1]:0"
	}
	listener, err := net.Listen(network, address) //nolint:noctx // Test listener creation has no context-aware variant.
	require.NoError(t, err)
	server := &httptest.Server{ //nolint:exhaustruct // Test server uses a family-specific listener.
		Listener: listener,
		Config: &http.Server{ //nolint:exhaustruct // Test server needs only a handler and read-header timeout.
			Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				_, _ = fmt.Fprint(w, response(req))
			}),
			ReadHeaderTimeout: time.Minute,
		},
	}
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func TestAttemptCloudflareTraceValid(t *testing.T) {
	t.Parallel()

	server := newTraceAttemptServer(t, ipnet.IP4, func(req *http.Request) string {
		return fmt.Sprintf("h=%s\nip=192.0.2.1\nwarp=off\n", req.Host)
	})

	result := attemptCloudflareTrace(context.Background(), server.URL, ipnet.IP4, 24)

	require.Equal(t, traceAttemptSucceeded, result.status)
	require.Equal(t, NewKnownDetectionResult([]ipnet.RawEntry{
		ipnet.RawEntryFrom(netip.MustParseAddr("192.0.2.1"), 24),
	}), result.rawData)
	require.Empty(t, result.warnings)
	require.Equal(t, traceFailure{}, result.failure) //nolint:exhaustruct // The zero value means no failure.
}

func TestAttemptCloudflareTraceTransportFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	traceURL := server.URL
	server.Close()

	result := attemptCloudflareTrace(context.Background(), traceURL, ipnet.IP4, 32)

	require.Equal(t, traceAttemptFailed, result.status)
	require.Equal(t, NewUnavailableDetectionResult(), result.rawData)
	require.Empty(t, result.warnings)
	require.Equal(t, traceFailureRequest, result.failure.kind)
	require.Error(t, result.failure.cause)
	require.Empty(t, result.failure.observed)
	require.Empty(t, result.failure.expected)
	require.Empty(t, result.failure.problem)
	require.False(t, result.failure.wantsMapped4Hint)
}

func TestAttemptCloudflareTraceCancellation(t *testing.T) {
	t.Parallel()

	server := newTraceAttemptServer(t, ipnet.IP4, func(req *http.Request) string {
		return fmt.Sprintf("h=%s\nip=192.0.2.1\nwarp=off\n", req.Host)
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := attemptCloudflareTrace(ctx, server.URL, ipnet.IP4, 32)

	require.Equal(t, traceAttemptCanceled, result.status)
	require.Equal(t, NewUnavailableDetectionResult(), result.rawData)
	require.Empty(t, result.warnings)
	require.Equal(t, traceFailure{}, result.failure) //nolint:exhaustruct // Cancellation is not a definite failure.
}

func TestAttemptCloudflareTraceMissingHWarning(t *testing.T) {
	t.Parallel()

	server := newTraceAttemptServer(t, ipnet.IP4, func(*http.Request) string {
		return "ip=192.0.2.1\nwarp=off\n"
	})

	result := attemptCloudflareTrace(context.Background(), server.URL, ipnet.IP4, 32)

	require.Equal(t, traceAttemptSucceeded, result.status)
	require.Equal(t, NewKnownDetectionResult([]ipnet.RawEntry{
		ipnet.RawEntryFrom(netip.MustParseAddr("192.0.2.1"), 32),
	}), result.rawData)
	require.Equal(t, []traceWarningKind{traceWarningMissingH}, result.warnings)
	require.Equal(t, traceFailure{}, result.failure) //nolint:exhaustruct // The zero value means no failure.
}

func TestAttemptCloudflareTraceMismatchedH(t *testing.T) {
	t.Parallel()

	server := newTraceAttemptServer(t, ipnet.IP4, func(*http.Request) string {
		return "h=wrong.example.com\nip=192.0.2.1\nwarp=off\n"
	})

	result := attemptCloudflareTrace(context.Background(), server.URL, ipnet.IP4, 32)

	require.Equal(t, traceAttemptFailed, result.status)
	require.Equal(t, NewUnavailableDetectionResult(), result.rawData)
	require.Empty(t, result.warnings)
	require.Equal(t, traceFailureMismatchedH, result.failure.kind)
	require.NoError(t, result.failure.cause)
	require.Equal(t, "wrong.example.com", result.failure.observed)
	expectedHost := server.Listener.Addr().String()
	require.Equal(t, expectedHost, result.failure.expected)
	require.Empty(t, result.failure.problem)
	require.False(t, result.failure.wantsMapped4Hint)
}

func TestAttemptCloudflareTraceMissingWarpWarning(t *testing.T) {
	t.Parallel()

	server := newTraceAttemptServer(t, ipnet.IP4, func(req *http.Request) string {
		return fmt.Sprintf("h=%s\nip=192.0.2.1\n", req.Host)
	})

	result := attemptCloudflareTrace(context.Background(), server.URL, ipnet.IP4, 32)

	require.Equal(t, traceAttemptSucceeded, result.status)
	require.Equal(t, NewKnownDetectionResult([]ipnet.RawEntry{
		ipnet.RawEntryFrom(netip.MustParseAddr("192.0.2.1"), 32),
	}), result.rawData)
	require.Equal(t, []traceWarningKind{traceWarningMissingWarp}, result.warnings)
	require.Equal(t, traceFailure{}, result.failure) //nolint:exhaustruct // The zero value means no failure.
}

func TestAttemptCloudflareTraceWarpOn(t *testing.T) {
	t.Parallel()

	server := newTraceAttemptServer(t, ipnet.IP4, func(req *http.Request) string {
		return fmt.Sprintf("h=%s\nip=192.0.2.1\nwarp=on\n", req.Host)
	})

	result := attemptCloudflareTrace(context.Background(), server.URL, ipnet.IP4, 32)

	require.Equal(t, traceAttemptFailed, result.status)
	require.Equal(t, NewUnavailableDetectionResult(), result.rawData)
	require.Empty(t, result.warnings)
	require.Equal(t, traceFailureWarpOn, result.failure.kind)
	require.NoError(t, result.failure.cause)
	require.Equal(t, "on", result.failure.observed)
	require.Empty(t, result.failure.expected)
	require.Empty(t, result.failure.problem)
	require.False(t, result.failure.wantsMapped4Hint)
}

func TestAttemptCloudflareTraceMissingIP(t *testing.T) {
	t.Parallel()

	server := newTraceAttemptServer(t, ipnet.IP4, func(req *http.Request) string {
		return fmt.Sprintf("h=%s\nwarp=off\n", req.Host)
	})

	result := attemptCloudflareTrace(context.Background(), server.URL, ipnet.IP4, 32)

	require.Equal(t, traceAttemptFailed, result.status)
	require.Equal(t, NewUnavailableDetectionResult(), result.rawData)
	require.Empty(t, result.warnings)
	require.Equal(t, traceFailureMissingIP, result.failure.kind)
	require.NoError(t, result.failure.cause)
	require.Empty(t, result.failure.observed)
	require.Empty(t, result.failure.expected)
	require.Empty(t, result.failure.problem)
	require.False(t, result.failure.wantsMapped4Hint)
}

func TestAttemptCloudflareTraceUnparseableIP(t *testing.T) {
	t.Parallel()

	server := newTraceAttemptServer(t, ipnet.IP4, func(req *http.Request) string {
		return fmt.Sprintf("h=%s\nip=not-an-ip\nwarp=off\n", req.Host)
	})

	result := attemptCloudflareTrace(context.Background(), server.URL, ipnet.IP4, 32)

	require.Equal(t, traceAttemptFailed, result.status)
	require.Equal(t, NewUnavailableDetectionResult(), result.rawData)
	require.Empty(t, result.warnings)
	require.Equal(t, traceFailureUnparseableIP, result.failure.kind)
	require.NoError(t, result.failure.cause)
	require.Equal(t, "not-an-ip", result.failure.observed)
	require.Empty(t, result.failure.expected)
	require.Empty(t, result.failure.problem)
	require.False(t, result.failure.wantsMapped4Hint)
}

func TestAttemptCloudflareTraceCloudflareRange(t *testing.T) {
	t.Parallel()

	server := newTraceAttemptServer(t, ipnet.IP4, func(req *http.Request) string {
		return fmt.Sprintf("h=%s\nip=104.16.0.1\nwarp=off\n", req.Host)
	})

	result := attemptCloudflareTrace(context.Background(), server.URL, ipnet.IP4, 32)

	require.Equal(t, traceAttemptFailed, result.status)
	require.Equal(t, NewUnavailableDetectionResult(), result.rawData)
	require.Empty(t, result.warnings)
	require.Equal(t, traceFailureCloudflareIP, result.failure.kind)
	require.NoError(t, result.failure.cause)
	require.Equal(t, "104.16.0.1", result.failure.observed)
	require.Empty(t, result.failure.expected)
	require.Empty(t, result.failure.problem)
	require.False(t, result.failure.wantsMapped4Hint)
}

func TestAttemptCloudflareTraceFamilyMismatch(t *testing.T) {
	t.Parallel()

	server := newTraceAttemptServer(t, ipnet.IP4, func(req *http.Request) string {
		return fmt.Sprintf("h=%s\nip=2001:db8::1\nwarp=off\n", req.Host)
	})

	result := attemptCloudflareTrace(context.Background(), server.URL, ipnet.IP4, 32)

	require.Equal(t, traceAttemptFailed, result.status)
	require.Equal(t, NewUnavailableDetectionResult(), result.rawData)
	require.Empty(t, result.warnings)
	require.Equal(t, traceFailureInvalidDetectedIP, result.failure.kind)
	require.NoError(t, result.failure.cause)
	require.Equal(t, "2001:db8::1", result.failure.observed)
	require.Empty(t, result.failure.expected)
	require.Equal(t, "is not a valid IPv4 address", result.failure.problem)
	require.False(t, result.failure.wantsMapped4Hint)
}

func TestAttemptCloudflareTraceMappedIPv6Hint(t *testing.T) {
	t.Parallel()

	server := newTraceAttemptServer(t, ipnet.IP6, func(req *http.Request) string {
		return fmt.Sprintf("h=%s\nip=::ffff:192.0.2.1\nwarp=off\n", req.Host)
	})

	result := attemptCloudflareTrace(context.Background(), server.URL, ipnet.IP6, 128)

	require.Equal(t, traceAttemptFailed, result.status)
	require.Equal(t, NewUnavailableDetectionResult(), result.rawData)
	require.Empty(t, result.warnings)
	require.Equal(t, traceFailureInvalidDetectedIP, result.failure.kind)
	require.NoError(t, result.failure.cause)
	require.Equal(t, "::ffff:192.0.2.1", result.failure.observed)
	require.Empty(t, result.failure.expected)
	require.Equal(t, "is an IPv4-mapped IPv6 address", result.failure.problem)
	require.True(t, result.failure.wantsMapped4Hint)
}

func TestDescribeCloudflareTraceFailureUnknownKind(t *testing.T) {
	t.Parallel()

	failure := traceFailure{ //nolint:exhaustruct // An unknown kind carries no recognized failure details.
		kind: traceFailureKind(255),
	}

	// Mutation caught: removing or changing the safe fallback for an unrecognized internal failure kind.
	require.Equal(t, "unknown Cloudflare trace failure", describeCloudflareTraceFailure(failure))
}

func TestReportCloudflareTraceWinnerWarningsIgnoresNoRecognizedWarnings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		warnings []traceWarningKind
	}{
		{name: "empty", warnings: nil},
		{name: "unknown", warnings: []traceWarningKind{traceWarningKind(255)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var output strings.Builder
			reportCloudflareTraceWinnerWarnings(
				pp.New(&output, false, pp.Verbose),
				ipnet.IP4,
				"primary",
				"https://example.com/cdn-cgi/trace",
				test.warnings,
			)

			// Mutation caught: emitting a malformed winner warning when no recognized field is missing.
			require.Empty(t, output.String())
		})
	}
}
