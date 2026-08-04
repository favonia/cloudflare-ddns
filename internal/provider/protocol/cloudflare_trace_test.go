package protocol_test

// vim: nowrap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func TestCloudflareTraceName(t *testing.T) {
	t.Parallel()

	p := protocol.CloudflareTrace{
		ProviderName: "very secret name",
		URLs:         nil,
	}

	require.Equal(t, "very secret name", p.Name())
}

func TestCloudflareTraceIsExplicitEmpty(t *testing.T) {
	t.Parallel()

	require.False(t, protocol.CloudflareTrace{
		ProviderName: "",
		URLs:         nil,
	}.IsExplicitEmpty())
}

// hostFromURL extracts the Host field from a URL string for use in trace h= fields.
func hostFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

func expectCloudflareTraceFailure(
	m *mocks.MockPP, emoji pp.Emoji, ipFamily ipnet.Family, traceURL string, failure any,
) {
	m.EXPECT().Noticef(emoji,
		"Cloudflare trace %s detection via %s failed: %s",
		ipFamily.Describe(), pp.QuoteIfUnsafeInSentence(traceURL), failure)
}

func cloudflareTraceTestEndpoints(serverURL string) []string {
	return []string{
		serverURL + "/primary",
		serverURL + "/fallback",
		serverURL + "/tertiary",
	}
}

func cloudflareTraceTestEndpointIndex(path string) int {
	switch path {
	case "/primary":
		return 0
	case "/fallback":
		return 1
	case "/tertiary":
		return 2
	default:
		return -1
	}
}

func cloudflareTraceTestProvider(endpoints []string) protocol.CloudflareTrace {
	return protocol.CloudflareTrace{
		ProviderName: "test",
		URLs:         map[ipnet.Family][]string{ipnet.IP4: endpoints},
	}
}

func validCloudflareTraceResponse(req *http.Request) string {
	return fmt.Sprintf("h=%s\nip=192.0.2.1\nwarp=off\n", req.Host)
}

func TestCloudflareTraceGetRawDataValidatesPrimarySuccess(t *testing.T) {
	t.Parallel()

	var counts [3]atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		index := cloudflareTraceTestEndpointIndex(req.URL.Path)
		if index < 0 {
			t.Errorf("unexpected test endpoint %q", req.URL.Path)
			return
		}
		counts[index].Add(1)
		if index > 0 {
			<-req.Context().Done()
			return
		}
		_, _ = fmt.Fprint(w, validCloudflareTraceResponse(req))
	}))
	t.Cleanup(server.Close)

	result := cloudflareTraceTestProvider(cloudflareTraceTestEndpoints(server.URL)).
		GetRawData(context.Background(), pp.NewSilent(), ipnet.IP4, 32)

	// Mutation caught: failing to transmit or validate a clean primary response.
	// The synctest scheduler suite owns the no-hedge-before-ready-primary invariant.
	require.Equal(t, protocol.NewKnownDetectionResult([]ipnet.RawEntry{
		ipnet.RawEntryFrom(netip.MustParseAddr("192.0.2.1"), 32),
	}), result)
	require.Equal(t, int32(1), counts[0].Load())
	require.LessOrEqual(t, counts[1].Load(), int32(1))
	require.LessOrEqual(t, counts[2].Load(), int32(1))
}

func TestCloudflareTraceGetRawDataUsesFallbackAfterPrimaryFailure(t *testing.T) {
	t.Parallel()

	var counts [3]atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		index := cloudflareTraceTestEndpointIndex(req.URL.Path)
		if index < 0 {
			t.Errorf("unexpected test endpoint %q", req.URL.Path)
			return
		}
		counts[index].Add(1)
		switch index {
		case 0:
			//nolint:gosec // The httptest request host is required by the trace-response fixture.
			_, _ = fmt.Fprintf(w, "h=%s\nwarp=off\n", req.Host)
		case 1:
			_, _ = fmt.Fprint(w, validCloudflareTraceResponse(req))
		case 2:
			<-req.Context().Done()
		}
	}))
	t.Cleanup(server.Close)

	result := cloudflareTraceTestProvider(cloudflareTraceTestEndpoints(server.URL)).
		GetRawData(context.Background(), pp.NewSilent(), ipnet.IP4, 32)

	// Mutation caught: failing to use a valid fallback after a definite primary failure.
	// The synctest scheduler suite owns failure-acceleration timing and launch order.
	require.True(t, result.Available)
	require.Equal(t, int32(1), counts[0].Load())
	require.Equal(t, int32(1), counts[1].Load())
	require.LessOrEqual(t, counts[2].Load(), int32(1))
}

func TestCloudflareTraceGetRawDataAttemptsEachEndpointOnce(t *testing.T) {
	t.Parallel()

	var counts [3]atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		index := cloudflareTraceTestEndpointIndex(req.URL.Path)
		if index < 0 {
			t.Errorf("unexpected test endpoint %q", req.URL.Path)
			return
		}
		counts[index].Add(1)
		//nolint:gosec // The httptest request host is required by the trace-response fixture.
		_, _ = fmt.Fprintf(w, "h=%s\nwarp=off\n", req.Host)
	}))
	t.Cleanup(server.Close)

	result := cloudflareTraceTestProvider(cloudflareTraceTestEndpoints(server.URL)).
		GetRawData(context.Background(), pp.NewSilent(), ipnet.IP4, 32)

	// Mutation caught: retrying an endpoint or omitting a configured endpoint after all failures.
	require.False(t, result.Available)
	for index := range counts {
		require.Equal(t, int32(1), counts[index].Load())
	}
}

func TestCloudflareTraceGetRawDataHidesLosingDiagnosticsAfterSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch cloudflareTraceTestEndpointIndex(req.URL.Path) {
		case 0:
			_, _ = fmt.Fprint(w, "warp=off\n")
		case 1:
			_, _ = fmt.Fprint(w, validCloudflareTraceResponse(req))
		case 2:
			<-req.Context().Done()
		default:
			t.Errorf("unexpected test endpoint %q", req.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	var output strings.Builder
	result := cloudflareTraceTestProvider(cloudflareTraceTestEndpoints(server.URL)).
		GetRawData(context.Background(), pp.New(&output, false, pp.Verbose), ipnet.IP4, 32)

	// Mutation caught: replaying warnings or terminal failures from a losing attempt after success.
	require.True(t, result.Available)
	require.NotContains(t, output.String(), "does not contain an h")
	require.NotContains(t, output.String(), "failed:")
	require.Contains(t, output.String(), "used fallback endpoint "+server.URL+"/fallback")
}

func TestCloudflareTraceGetRawDataReplaysWinnerWarnings(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch cloudflareTraceTestEndpointIndex(req.URL.Path) {
		case 0:
			//nolint:gosec // The httptest request host is required by the trace-response fixture.
			_, _ = fmt.Fprintf(w, "h=%s\nwarp=off\n", req.Host)
		case 1:
			_, _ = fmt.Fprint(w, "ip=192.0.2.1\n")
		case 2:
			<-req.Context().Done()
		default:
			t.Errorf("unexpected test endpoint %q", req.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	var output strings.Builder
	result := cloudflareTraceTestProvider(cloudflareTraceTestEndpoints(server.URL)).
		GetRawData(context.Background(), pp.New(&output, false, pp.Verbose), ipnet.IP4, 32)
	transcript := output.String()
	winnerURL := server.URL + "/fallback"

	// Mutation caught: splitting one warning-bearing success into multiple messages,
	// choosing the wrong endpoint role, or repeating the issue-reporting action.
	require.True(t, result.Available)
	require.Equal(t,
		"Cloudflare trace IPv4 detection succeeded via fallback endpoint "+winnerURL+
			", but its response is missing the h (host) and warp fields; please report this at "+
			"https://github.com/favonia/cloudflare-ddns/issues/new/choose\n",
		transcript,
	)
	require.Equal(t, 1, strings.Count(transcript, "please report this at"))
}

func TestCloudflareTraceGetRawDataReportsMissingWarpFromWinner(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if cloudflareTraceTestEndpointIndex(req.URL.Path) != 0 {
			<-req.Context().Done()
			return
		}
		//nolint:gosec // The httptest request host is required by the trace-response fixture.
		_, _ = fmt.Fprintf(w, "h=%s\nip=192.0.2.1\n", req.Host)
	}))
	t.Cleanup(server.Close)

	var output strings.Builder
	result := cloudflareTraceTestProvider(cloudflareTraceTestEndpoints(server.URL)).
		GetRawData(context.Background(), pp.New(&output, false, pp.Verbose), ipnet.IP4, 32)

	// Mutation caught: dropping or mislabeling the sole missing-warp warning from a successful primary response.
	require.True(t, result.Available)
	require.Equal(t,
		"Cloudflare trace IPv4 detection succeeded via primary endpoint "+server.URL+
			"/primary, but its response is missing the warp field; please report this at "+
			"https://github.com/favonia/cloudflare-ddns/issues/new/choose\n",
		output.String(),
	)
}

func TestCloudflareTraceGetRawDataReportsFailuresInEndpointOrder(t *testing.T) {
	t.Parallel()

	started := [3]chan struct{}{make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{}, 1)}
	release := [3]chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	var releaseOnce [3]sync.Once
	releaseEndpoint := func(index int) {
		releaseOnce[index].Do(func() { close(release[index]) })
	}
	completed := [3]chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	completionOrder := make([]string, 0, 3)
	responses := []string{
		"h=%s\nwarp=off\n",
		"h=%s\nip=192.0.2.1\nwarp=on\n",
		"h=%s\nip=not-an-ip\nwarp=off\n",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		index := cloudflareTraceTestEndpointIndex(req.URL.Path)
		if index < 0 {
			t.Errorf("unexpected test endpoint %q", req.URL.Path)
			return
		}
		started[index] <- struct{}{}
		<-release[index]
		//nolint:gosec // The httptest request host is required by the trace-response fixture.
		_, _ = fmt.Fprintf(w, responses[index], req.Host)
		completionOrder = append(completionOrder, req.URL.Path)
		close(completed[index])
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		for index := range release {
			releaseEndpoint(index)
		}
	})
	endpoints := cloudflareTraceTestEndpoints(server.URL)
	var output strings.Builder
	resultChannel := make(chan protocol.DetectionResult, 1)
	go func() {
		resultChannel <- cloudflareTraceTestProvider(endpoints).
			GetRawData(ctx, pp.New(&output, false, pp.Verbose), ipnet.IP4, 32)
	}()
	for index := range started {
		select {
		case <-started[index]:
		case <-time.After(2 * time.Second):
			require.FailNow(t, "endpoint was not started", "endpoint %s", endpoints[index])
		}
	}
	for _, index := range []int{2, 1, 0} {
		releaseEndpoint(index)
		select {
		case <-completed[index]:
		case <-time.After(2 * time.Second):
			require.FailNow(t, "endpoint did not complete", "endpoint %s", endpoints[index])
		}
	}
	var result protocol.DetectionResult
	select {
	case result = <-resultChannel:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "Cloudflare trace coordinator did not complete")
	}
	transcript := output.String()

	// Mutation caught: omitting a definite failure or rendering failures in worker completion order.
	require.False(t, result.Available)
	require.Equal(t, []string{"/tertiary", "/fallback", "/primary"}, completionOrder)
	offsets := [3]int{
		strings.Index(transcript, endpoints[0]),
		strings.Index(transcript, endpoints[1]),
		strings.Index(transcript, endpoints[2]),
	}
	for index, offset := range offsets {
		require.NotEqual(t, -1, offset, "endpoint index %d", index)
	}
	require.Less(t, offsets[0], offsets[1])
	require.Less(t, offsets[1], offsets[2])
}

func TestCloudflareTraceGetRawDataReportsSharedTimeoutOnce(t *testing.T) {
	t.Parallel()

	var counts [3]atomic.Int32
	requestObserved := make(chan struct{}, 3)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		index := cloudflareTraceTestEndpointIndex(req.URL.Path)
		if index < 0 {
			t.Errorf("unexpected test endpoint %q", req.URL.Path)
			return
		}
		counts[index].Add(1)
		requestObserved <- struct{}{}
		<-req.Context().Done()
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(context.Canceled)
	var output strings.Builder
	resultChannel := make(chan protocol.DetectionResult, 1)
	go func() {
		resultChannel <- cloudflareTraceTestProvider(cloudflareTraceTestEndpoints(server.URL)).
			GetRawData(ctx, pp.New(&output, false, pp.Verbose), ipnet.IP4, 32)
	}()
	<-requestObserved
	cancel(context.DeadlineExceeded)
	result := <-resultChannel
	transcript := output.String()

	// Mutation caught: rendering a controlled DeadlineExceeded cancellation as worker errors
	// instead of one operation-level timeout. The synctest scheduler suite owns launch timing.
	require.False(t, result.Available)
	require.Equal(t, 1, strings.Count(transcript, "timed out before any endpoint returned a valid response"))
	require.NotContains(t, transcript, "context deadline exceeded")
	require.Equal(t, int32(1), counts[0].Load())
	require.LessOrEqual(t, counts[1].Load(), int32(1))
	require.LessOrEqual(t, counts[2].Load(), int32(1))
}

func TestCloudflareTraceGetRawDataReportsMappedIPv6HintOnceAfterEndpointFailures(t *testing.T) {
	t.Parallel()

	var counts [3]atomic.Int32
	server := newSplitServer(ipnet.IP6, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		index := cloudflareTraceTestEndpointIndex(req.URL.Path)
		if index < 0 {
			t.Errorf("unexpected test endpoint %q", req.URL.Path)
			return
		}
		counts[index].Add(1)
		switch index {
		case 0:
			//nolint:gosec // The httptest request host is required by the trace-response fixture.
			_, _ = fmt.Fprintf(w, "h=%s\nwarp=off\n", req.Host)
		case 1:
			//nolint:gosec // The httptest request host is required by the trace-response fixture.
			_, _ = fmt.Fprintf(w, "h=%s\nip=::ffff:192.0.2.1\nwarp=off\n", req.Host)
		case 2:
			//nolint:gosec // The httptest request host is required by the trace-response fixture.
			_, _ = fmt.Fprintf(w, "h=%s\nip=not-an-ip\nwarp=off\n", req.Host)
		}
	}))
	t.Cleanup(server.Close)

	endpoints := cloudflareTraceTestEndpoints(server.URL)
	provider := protocol.CloudflareTrace{
		ProviderName: "test",
		URLs:         map[ipnet.Family][]string{ipnet.IP6: endpoints},
	}
	var output strings.Builder
	result := provider.GetRawData(context.Background(), pp.New(&output, false, pp.Verbose), ipnet.IP6, 128)
	transcript := output.String()
	hint := "An IPv4-mapped IPv6 address is an IPv4 address in disguise."

	// Mutation caught: dropping the aggregated mapped-address hint, repeating it per
	// endpoint, or rendering it before all configured endpoint failure summaries.
	require.False(t, result.Available)
	for index := range counts {
		require.Equal(t, int32(1), counts[index].Load(), "endpoint index %d", index)
		require.Contains(t, transcript, endpoints[index])
		require.Less(t, strings.Index(transcript, endpoints[index]), strings.Index(transcript, hint))
	}
	require.Equal(t, 1, strings.Count(transcript, hint))
}

func TestCloudflareTraceGetRawData(t *testing.T) {
	t.Parallel()

	ip4 := netip.MustParseAddr("1.2.3.4")
	ip6 := netip.MustParseAddr("::1:2:3:4:5:6")

	type testCase struct {
		ipFamily      ipnet.Family
		serverFamily  ipnet.Family
		makeResponse  func(serverURL string) string
		noServer      bool   // skip creating a test server
		forceURL      string // override URL for the provider
		unmappedIP    ipnet.Family
		available     bool
		expected      netip.Addr
		prepareMockPP func(serverURL string, m *mocks.MockPP)
	}

	for name, tc := range map[string]testCase{
		"4/valid": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=off\n", hostFromURL(serverURL), ip4)
			},
			available: true, expected: ip4,
		},
		"6/valid": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=off\n", hostFromURL(serverURL), ip6)
			},
			available: true, expected: ip6,
		},
		"4/missing-h-warns": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(_ string) string {
				return fmt.Sprintf("ip=%s\nwarp=off\n", ip4)
			},
			available: true, expected: ip4,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible,
					"Cloudflare trace %s detection succeeded via %s endpoint %s, "+
						"but its response is missing %s; please report this at %s",
					"IPv4", "primary", serverURL, "the h (host) field", pp.IssueReportingURL)
			},
		},
		"4/mismatched-h": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(_ string) string {
				return fmt.Sprintf("h=wrong.example.com\nip=%s\nwarp=off\n", ip4)
			},
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiImpossible, ipnet.IP4, serverURL, fmt.Sprintf(
					"the h field %q does not match the expected host %q; please report this at %s",
					"wrong.example.com", hostFromURL(serverURL), pp.IssueReportingURL))
			},
		},
		"4/warp-on": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=on\n", hostFromURL(serverURL), ip4)
			},
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiError, ipnet.IP4, serverURL,
					"the response has warp=on; the detected IP is a Cloudflare WARP egress IP, not your real public IP")
			},
		},
		"4/missing-warp-warns": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\n", hostFromURL(serverURL), ip4)
			},
			available: true, expected: ip4,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible,
					"Cloudflare trace %s detection succeeded via %s endpoint %s, "+
						"but its response is missing %s; please report this at %s",
					"IPv4", "primary", serverURL, "the warp field", pp.IssueReportingURL)
			},
		},
		"4/missing-ip": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nwarp=off\n", hostFromURL(serverURL))
			},
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiError, ipnet.IP4, serverURL,
					"the response does not contain an ip field")
			},
		},
		"4/unparseable-ip": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=not-an-ip\nwarp=off\n", hostFromURL(serverURL))
			},
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiError, ipnet.IP4, serverURL,
					`failed to parse the IP address "not-an-ip"`)
			},
		},
		"4/cloudflare-ipv4-range": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=104.16.0.1\nwarp=off\n", hostFromURL(serverURL))
			},
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiError, ipnet.IP4, serverURL,
					"the detected IP address 104.16.0.1 is inside Cloudflare's own IP range and is not your real public IP")
			},
		},
		"6/cloudflare-ipv6-range": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=2606:4700::1\nwarp=off\n", hostFromURL(serverURL))
			},
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiError, ipnet.IP6, serverURL,
					"the detected IP address 2606:4700::1 is inside Cloudflare's own IP range and is not your real public IP")
			},
		},
		"4/not-handled": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			unmappedIP:   ipnet.IP4, // provider will have IP6 entry only
			makeResponse: func(_ string) string { return "" },
			available:    false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", "IPv4")
			},
		},
		"4/ip6-response-family-mismatch": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=off\n", hostFromURL(serverURL), ip6)
			},
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiError, ipnet.IP4, serverURL,
					"the detected IP address ::1:2:3:4:5:6 is not a valid IPv4 address")
			},
		},
		"6/ip4-response-family-mismatch": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=off\n", hostFromURL(serverURL), ip4)
			},
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiError, ipnet.IP6, serverURL,
					"the detected IP address 1.2.3.4 is not a valid IPv6 address")
			},
		},
		"4/extra-fields-ignored": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("fl=abc123\nh=%s\nip=%s\nts=1234567890\nwarp=off\ncolo=SJC\n", hostFromURL(serverURL), ip4)
			},
			available: true, expected: ip4,
		},
		"4/empty-response": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(_ string) string { return "" },
			available:    false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiError, ipnet.IP4, serverURL,
					"the response does not contain an ip field")
			},
		},
		"4/lines-without-equals": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("some-garbage\nh=%s\nip=%s\nwarp=off\nanother-line\n", hostFromURL(serverURL), ip4)
			},
			available: true, expected: ip4,
		},
		"6/warp-on": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=on\n", hostFromURL(serverURL), ip6)
			},
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiError, ipnet.IP6, serverURL,
					"the response has warp=on; the detected IP is a Cloudflare WARP egress IP, not your real public IP")
			},
		},
		"4/warp-plus-passes": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, serverFamily: ipnet.IP4,
			makeResponse: func(serverURL string) string {
				return fmt.Sprintf("h=%s\nip=%s\nwarp=plus\n", hostFromURL(serverURL), ip4)
			},
			available: true, expected: ip4,
		},
		"6/missing-h-warns": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(_ string) string {
				return fmt.Sprintf("ip=%s\nwarp=off\n", ip6)
			},
			available: true, expected: ip6,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				displayServerURL := fmt.Sprintf("%q", serverURL)
				m.EXPECT().Noticef(pp.EmojiImpossible,
					"Cloudflare trace %s detection succeeded via %s endpoint %s, "+
						"but its response is missing %s; please report this at %s",
					"IPv6", "primary", displayServerURL, "the h (host) field", pp.IssueReportingURL)
			},
		},
		"6/mismatched-h": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			makeResponse: func(_ string) string {
				return fmt.Sprintf("h=wrong.example.com\nip=%s\nwarp=off\n", ip6)
			},
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiImpossible, ipnet.IP6, serverURL, fmt.Sprintf(
					"the h field %q does not match the expected host %q; please report this at %s",
					"wrong.example.com", hostFromURL(serverURL), pp.IssueReportingURL))
			},
		},
		"6/not-handled": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, serverFamily: ipnet.IP6,
			unmappedIP:   ipnet.IP6,
			makeResponse: func(_ string) string { return "" },
			available:    false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "Unhandled IP family: %s", "IPv6")
			},
		},
		"4/request-fail": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP4, noServer: true, forceURL: "",
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiError, ipnet.IP4, serverURL, gomock.Any())
			},
		},
		"6/request-fail": { //nolint:exhaustruct // test fixture sets only exercised fields
			ipFamily: ipnet.IP6, noServer: true, forceURL: "",
			available: false,
			prepareMockPP: func(serverURL string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiError, ipnet.IP6, serverURL, gomock.Any())
			},
		},
		"4/illegal-url-escape": { //nolint:exhaustruct // test fixture sets only exercised fields
			// A URL with an illegal percent-escape fails endpoint parsing before transmission.
			ipFamily: ipnet.IP4, noServer: true, forceURL: "http://example.com/path%zz",
			available: false,
			prepareMockPP: func(_ string, m *mocks.MockPP) {
				expectCloudflareTraceFailure(m, pp.EmojiImpossible, ipnet.IP4,
					"http://example.com/path%zz", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)

			var provider protocol.CloudflareTrace
			var serverURL string

			if tc.noServer {
				provider = protocol.CloudflareTrace{
					ProviderName: "test",
					URLs:         map[ipnet.Family][]string{tc.ipFamily: {tc.forceURL}},
				}
			} else {
				var server *httptest.Server
				server = newSplitServer(tc.serverFamily, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					fmt.Fprint(w, tc.makeResponse(server.URL))
				}))
				t.Cleanup(server.Close)
				serverURL = server.URL

				if tc.unmappedIP == tc.ipFamily {
					// Map to opposite family so this family is unhandled.
					other := ipnet.IP6
					if tc.ipFamily == ipnet.IP6 {
						other = ipnet.IP4
					}
					provider = protocol.CloudflareTrace{
						ProviderName: "test",
						URLs:         map[ipnet.Family][]string{other: {server.URL}},
					}
				} else {
					provider = protocol.CloudflareTrace{
						ProviderName: "test",
						URLs:         map[ipnet.Family][]string{tc.ipFamily: {server.URL}},
					}
				}
			}

			if tc.prepareMockPP != nil {
				tc.prepareMockPP(serverURL, mockPP)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			prefixLen := testDefaultPrefixLen(tc.ipFamily)
			rawData := provider.GetRawData(ctx, mockPP, tc.ipFamily, prefixLen)
			require.Equal(t, tc.available, rawData.Available)
			if tc.expected.IsValid() {
				require.Equal(t, []ipnet.RawEntry{ipnet.RawEntryFrom(tc.expected, prefixLen)}, rawData.RawEntries)
			} else {
				require.Empty(t, rawData.RawEntries)
			}
		})
	}
}
