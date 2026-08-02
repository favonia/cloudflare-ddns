package updater

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipfilter"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

type cloudflareTraceTranscriptCapture struct {
	transcript     string
	heartbeat      string
	notifier       string
	endpoints      []string
	requestCounts  [3]int32
	winnerWarnings bool
}

func cloudflareTraceTranscriptEndpoints(serverURL string) []string {
	return []string{
		serverURL + "/primary",
		serverURL + "/fallback",
		serverURL + "/tertiary",
	}
}

func cloudflareTraceTranscriptEndpointIndex(path string) int {
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

func captureCloudflareTraceDetectionTranscript(
	t *testing.T, scenario string, verbosity pp.Verbosity,
) cloudflareTraceTranscriptCapture {
	t.Helper()

	var requestCounts [3]atomic.Int32
	requestObserved := make(chan struct{}, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		index := cloudflareTraceTranscriptEndpointIndex(req.URL.Path)
		if index < 0 {
			t.Errorf("unexpected transcript endpoint %q", req.URL.Path)
			return
		}
		requestCounts[index].Add(1)
		requestObserved <- struct{}{}

		switch scenario {
		case "primary-clean-success":
			if index > 0 {
				<-req.Context().Done()
				return
			}
			//nolint:gosec // The httptest request host is required by the trace-response fixture.
			_, _ = fmt.Fprintf(w, "h=%s\nip=203.0.113.7\nwarp=off\n", req.Host)

		case "fallback-clean-success":
			if index == 0 {
				//nolint:gosec // The httptest request host is required by the trace-response fixture.
				_, _ = fmt.Fprintf(w, "h=%s\nwarp=off\n", req.Host)
				return
			}
			if index > 1 {
				<-req.Context().Done()
				return
			}
			//nolint:gosec // The httptest request host is required by the trace-response fixture.
			_, _ = fmt.Fprintf(w, "h=%s\nip=203.0.113.7\nwarp=off\n", req.Host)

		case "primary-winner-missing-h", "primary-winner-missing-warp", "primary-winner-missing-both",
			"fallback-winner-missing-h", "fallback-winner-missing-warp", "fallback-winner-missing-both":
			isFallbackScenario := strings.HasPrefix(scenario, "fallback-")
			if isFallbackScenario && index == 0 {
				//nolint:gosec // The httptest request host is required by the trace-response fixture.
				_, _ = fmt.Fprintf(w, "h=%s\nwarp=off\n", req.Host)
				return
			}
			if (!isFallbackScenario && index > 0) || (isFallbackScenario && index > 1) {
				<-req.Context().Done()
				return
			}
			switch {
			case strings.HasSuffix(scenario, "-h"):
				_, _ = fmt.Fprint(w, "ip=203.0.113.7\nwarp=off\n")
			case strings.HasSuffix(scenario, "-warp"):
				//nolint:gosec // The httptest request host is required by the trace-response fixture.
				_, _ = fmt.Fprintf(w, "h=%s\nip=203.0.113.7\n", req.Host)
			case strings.HasSuffix(scenario, "-both"):
				_, _ = fmt.Fprint(w, "ip=203.0.113.7\n")
			}

		case "complete-mixed-failure":
			switch index {
			case 0:
				_, _ = fmt.Fprint(w, "warp=off\n")
			case 1:
				_, _ = fmt.Fprint(w, "h=wrong.example\nip=203.0.113.7\nwarp=off\n")
			case 2:
				//nolint:gosec // The httptest request host is required by the trace-response fixture.
				_, _ = fmt.Fprintf(w, "h=%s\nip=2001:db8::1\nwarp=off\n", req.Host)
			}

		case "shared-detection-timeout":
			<-req.Context().Done()

		default:
			t.Errorf("unknown transcript scenario %q", scenario)
		}
	}))
	t.Cleanup(server.Close)

	detectionTimeout := time.Hour
	if scenario == "shared-detection-timeout" {
		detectionTimeout = 100 * time.Millisecond
	}
	endpoints := cloudflareTraceTranscriptEndpoints(server.URL)
	conf := &config.UpdateConfig{ //nolint:exhaustruct // Transcript test needs only detection settings.
		Provider: map[ipnet.Family]provider.Provider{
			ipnet.IP4: protocol.CloudflareTrace{
				ProviderName: "cloudflare.trace",
				URLs:         map[ipnet.Family][]string{ipnet.IP4: endpoints},
			},
		},
		DetectionFilter:  map[ipnet.Family]ipfilter.Filter{ipnet.IP4: ipfilter.KeepAll()},
		DefaultPrefixLen: map[ipnet.Family]int{ipnet.IP4: 32},
		DetectionTimeout: detectionTimeout,
	}

	var output strings.Builder
	ppfmt := pp.New(&output, false, verbosity)
	var msg Message
	if scenario == "shared-detection-timeout" {
		providerCtx, cancelProvider := context.WithCancelCause(context.Background())
		defer cancelProvider(context.Canceled)
		rawDataChannel := make(chan provider.DetectionResult, 1)
		go func() {
			rawDataChannel <- conf.Provider[ipnet.IP4].GetRawData(
				providerCtx, ppfmt, ipnet.IP4, conf.DefaultPrefixLen[ipnet.IP4],
			)
		}()
		<-requestObserved
		cancelProvider(context.DeadlineExceeded)
		rawData := <-rawDataChannel

		renderCtx, cancelRender := context.WithCancelCause(context.Background())
		cancelRender(errTimeout)
		_, msg = finalizeDetectedRawData(renderCtx, ppfmt, conf, ipnet.IP4, rawData)
	} else {
		_, msg = detectRawData(context.Background(), ppfmt, conf, ipnet.IP4)
	}
	capture := cloudflareTraceTranscriptCapture{
		transcript:    output.String(),
		heartbeat:     msg.HeartbeatMessage.Format(),
		notifier:      msg.NotifierMessage.Format(),
		endpoints:     append([]string(nil), endpoints...),
		requestCounts: [3]int32{requestCounts[0].Load(), requestCounts[1].Load(), requestCounts[2].Load()},
		winnerWarnings: strings.Contains(output.String(), "response is missing the h (host)") ||
			strings.Contains(output.String(), "response is missing the warp field"),
	}

	return capture
}

func TestCloudflareTraceDetectionTranscript(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{
		"primary-clean-success",
		"fallback-clean-success",
		"complete-mixed-failure",
		"shared-detection-timeout",
	} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()

			for _, verbosity := range []pp.Verbosity{pp.Verbose, pp.Quiet} {
				capture := captureCloudflareTraceDetectionTranscript(t, scenario, verbosity)
				var wantTranscript string

				switch scenario {
				case "primary-clean-success":
					// Mutation caught: adding provider chatter to a clean primary success.
					require.Empty(t, capture.heartbeat)
					require.Empty(t, capture.notifier)
					require.Equal(t, int32(1), capture.requestCounts[0])
					require.LessOrEqual(t, capture.requestCounts[1], int32(1))
					require.LessOrEqual(t, capture.requestCounts[2], int32(1))
					require.False(t, capture.winnerWarnings)
					if verbosity == pp.Verbose {
						wantTranscript = "Detected IPv4 address: 203.0.113.7\n"
					}

				case "fallback-clean-success":
					// Mutation caught: claiming the primary failed, exposing loser diagnostics, or losing quiet suppression.
					require.Empty(t, capture.heartbeat)
					require.Empty(t, capture.notifier)
					require.Equal(t, int32(1), capture.requestCounts[0])
					require.Equal(t, int32(1), capture.requestCounts[1])
					require.LessOrEqual(t, capture.requestCounts[2], int32(1))
					require.False(t, capture.winnerWarnings)
					if verbosity == pp.Verbose {
						wantTranscript = fmt.Sprintf(
							"Cloudflare trace IPv4 detection used fallback endpoint %s\n"+
								"Detected IPv4 address: 203.0.113.7\n",
							capture.endpoints[1],
						)
					}

				case "complete-mixed-failure":
					// Mutation caught: omitting endpoint context, replaying failed-attempt warnings, or promising another run.
					require.Equal(t, "Failed to detect any IPv4 addresses", capture.heartbeat)
					require.Equal(t, "Failed to detect any IPv4 addresses.", capture.notifier)
					require.Equal(t, [3]int32{1, 1, 1}, capture.requestCounts)
					require.False(t, capture.winnerWarnings)
					host := strings.TrimSuffix(strings.TrimPrefix(capture.endpoints[0], "http://"), "/primary")
					wantTranscript = fmt.Sprintf(
						"Cloudflare trace IPv4 detection via %s failed: the response does not contain an ip field\n"+
							"Cloudflare trace IPv4 detection via %s failed: the h field \"wrong.example\" does not match the expected host \"%s\"; please report this at https://github.com/favonia/cloudflare-ddns/issues/new/choose\n"+
							"Cloudflare trace IPv4 detection via %s failed: the detected IP address 2001:db8::1 is not a valid IPv4 address\n"+
							"No valid IPv4 addresses were detected\n"+
							"If your network does not support IPv4, you can stop managing it with IP4_PROVIDER=none\n",
						capture.endpoints[0], capture.endpoints[1], host, capture.endpoints[2],
					)

				case "shared-detection-timeout":
					// Mutation caught: repeating worker deadlines or placing family guidance before timeout remediation.
					require.Equal(t, "Failed to detect any IPv4 addresses", capture.heartbeat)
					require.Equal(t, "Failed to detect any IPv4 addresses.", capture.notifier)
					require.Equal(t, int32(1), capture.requestCounts[0])
					require.LessOrEqual(t, capture.requestCounts[1], int32(1))
					require.LessOrEqual(t, capture.requestCounts[2], int32(1))
					require.False(t, capture.winnerWarnings)
					wantTranscript = "Cloudflare trace IPv4 detection timed out before any endpoint returned a valid response\n" +
						"No valid IPv4 addresses were detected\n" +
						"If your network is experiencing high latency, consider increasing DETECTION_TIMEOUT=100ms\n" +
						"If your network does not support IPv4, you can stop managing it with IP4_PROVIDER=none\n"
				}

				require.Equal(t, wantTranscript, capture.transcript)
			}
		})
	}

	for _, tc := range []struct {
		name          string
		winnerIndex   int
		endpointRole  string
		missingClause string
	}{
		{name: "primary-winner-missing-h", winnerIndex: 0, endpointRole: "primary", missingClause: "but its response is missing the h (host) field"},
		{name: "primary-winner-missing-warp", winnerIndex: 0, endpointRole: "primary", missingClause: "but its response is missing the warp field"},
		{name: "primary-winner-missing-both", winnerIndex: 0, endpointRole: "primary", missingClause: "but its response is missing the h (host) and warp fields"},
		{name: "fallback-winner-missing-h", winnerIndex: 1, endpointRole: "fallback", missingClause: "but its response is missing the h (host) field"},
		{name: "fallback-winner-missing-warp", winnerIndex: 1, endpointRole: "fallback", missingClause: "but its response is missing the warp field"},
		{name: "fallback-winner-missing-both", winnerIndex: 1, endpointRole: "fallback", missingClause: "but its response is missing the h (host) and warp fields"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, verbosity := range []pp.Verbosity{pp.Verbose, pp.Quiet} {
				capture := captureCloudflareTraceDetectionTranscript(t, tc.name, verbosity)
				winnerEndpoint := capture.endpoints[tc.winnerIndex]
				combinedNotice := fmt.Sprintf(
					"Cloudflare trace IPv4 detection succeeded via %s endpoint %s, %s; please report this at https://github.com/favonia/cloudflare-ddns/issues/new/choose\n",
					tc.endpointRole, winnerEndpoint, tc.missingClause,
				)
				wantTranscript := combinedNotice
				if verbosity == pp.Verbose {
					wantTranscript += "Detected IPv4 address: 203.0.113.7\n"
				}

				// Mutation caught: reporting warning-bearing success as separate context and warning lines,
				// choosing the wrong endpoint role or missing-field grammar, or repeating the issue action.
				require.Equal(t, wantTranscript, capture.transcript)
				require.Empty(t, capture.heartbeat)
				require.Empty(t, capture.notifier)
				require.True(t, capture.winnerWarnings)
				require.Equal(t, 1, strings.Count(capture.transcript, "please report this at"))
				if tc.winnerIndex == 0 {
					require.Equal(t, int32(1), capture.requestCounts[0])
					require.LessOrEqual(t, capture.requestCounts[1], int32(1))
					require.LessOrEqual(t, capture.requestCounts[2], int32(1))
				} else {
					require.Equal(t, int32(1), capture.requestCounts[0])
					require.Equal(t, int32(1), capture.requestCounts[1])
					require.LessOrEqual(t, capture.requestCounts[2], int32(1))
				}
			}
		})
	}
}

func TestClassifyNotification(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ok       bool
		wantKind notifier.Kind
	}{
		"success": {true, notifier.KindUpdate},
		"failure": {false, notifier.KindUpdateFailure},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			msg := Message{
				HeartbeatMessage: heartbeat.NewMessagef(tc.ok, "heartbeat"),
				NotifierMessage:  notifier.NewMessagef("notification"),
				NotificationKind: "",
			}
			got := classifyNotification(
				msg,
				notifier.KindUpdate,
				notifier.KindUpdateFailure,
			)
			require.Equal(t, tc.wantKind, got.NotificationKind)
			require.Equal(t,
				notifier.NewNotification(tc.wantKind, msg.NotifierMessage),
				got.Notification())
		})
	}
}

func TestSetIPsSkipsManagedDomainWithoutTargets(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(ctrl)
	s := mocks.NewMockSetter(ctrl)
	missing := domain.FQDN("missing.example")
	present := domain.FQDN("present.example")
	ip := netip.MustParseAddr("192.0.2.1")
	params := api.RecordParams{TTL: api.TTLAuto} //nolint:exhaustruct
	conf := &config.UpdateConfig{                //nolint:exhaustruct
		Domains:       map[ipnet.Family][]domain.Domain{ipnet.IP4: {missing, present}},
		Proxied:       map[domain.Domain]bool{},
		TTL:           api.TTLAuto,
		UpdateTimeout: time.Second,
	}

	gomock.InOrder(
		ppfmt.EXPECT().Noticef(pp.EmojiImpossible,
			"No target set was provided for managed domain %s; this should not happen. Please report it at %s",
			missing.Describe(), pp.IssueReportingURL),
		s.EXPECT().SetIPs(gomock.Any(), ppfmt, ipnet.IP4, present, []netip.Addr{ip}, params).
			Return(setter.ResponseUpdated),
	)

	msg := setIPs(context.Background(), ppfmt, conf, s, ipnet.IP4, dnsTargetsByDomain{
		present: {ip},
	})

	require.Equal(t, Message{
		HeartbeatMessage: heartbeat.Message{
			OK:    false,
			Lines: []string{"Could not update A records for missing.example"},
		},
		NotifierMessage: notifier.Message{
			"Could not update A records for missing.example because of an internal error; check the logs for details.",
			"Updated A records for present.example to 192.0.2.1.",
		},
		NotificationKind: "",
	}, msg)
}
