package heartbeat_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type heartbeatWithHTTPClient interface {
	heartbeat.BasicHeartbeat
	SetHTTPClient(client *http.Client)
}

type heartbeatHTTPTestCase struct {
	name           string
	responseBody   string
	defaultTimeout time.Duration
	newHeartbeat   func(*testing.T, string) heartbeatWithHTTPClient
}

// heartbeatHTTPTestCases constructs real heartbeats; each case owns its service
// response, default timeout, and constructor; tests share HTTP lifecycle assertions.
func heartbeatHTTPTestCases() []heartbeatHTTPTestCase {
	return []heartbeatHTTPTestCase{
		{
			name:           "healthchecks",
			responseBody:   "OK",
			defaultTimeout: heartbeat.HealthchecksDefaultTimeout,
			newHeartbeat: func(t *testing.T, url string) heartbeatWithHTTPClient {
				t.Helper()
				h, ok := heartbeat.NewHealthchecks(pp.NewSilent(), url)
				require.True(t, ok)
				return &h
			},
		},
		{
			name:           "uptime-kuma",
			responseBody:   `{"ok":true}`,
			defaultTimeout: heartbeat.UptimeKumaDefaultTimeout,
			newHeartbeat: func(t *testing.T, url string) heartbeatWithHTTPClient {
				t.Helper()
				h, ok := heartbeat.NewUptimeKuma(pp.NewSilent(), url)
				require.True(t, ok)
				return &h
			},
		},
	}
}

type heartbeatRoundTripper func(*http.Request) (*http.Response, error)

// RoundTrip closes the request body as required by http.RoundTripper.
func (f heartbeatRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		defer req.Body.Close()
	}
	return f(req)
}

// Exercise default transport selection through real heartbeat construction,
// without requiring net/http to reuse a particular TCP connection.
//
//nolint:paralleltest // Temporarily replaces http.DefaultTransport.
func TestHeartbeatUsesDefaultTransport(t *testing.T) {
	for _, tc := range heartbeatHTTPTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			requests := 0
			http.DefaultTransport = heartbeatRoundTripper(func(req *http.Request) (*http.Response, error) {
				requests++
				require.Equal(t, "heartbeat.example", req.URL.Host)
				return &http.Response{ //nolint:exhaustruct_v5 // Only these response fields are consumed by the client.
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(tc.responseBody)),
					Request:    req,
				}, nil
			})

			for range 2 {
				h := tc.newHeartbeat(t, "https://heartbeat.example")
				require.True(t, h.Ping(t.Context(), pp.NewSilent(), heartbeat.NewMessage()))
			}
			require.Equal(t, 2, requests)
		})
	}
}

func TestHeartbeatCancelsBlockedRequest(t *testing.T) {
	t.Parallel()
	for _, tc := range heartbeatHTTPTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				canceled := false
				client := &http.Client{ //nolint:exhaustruct_v5 // The test supplies only the blocking transport.
					Transport: heartbeatRoundTripper(func(req *http.Request) (*http.Response, error) {
						<-req.Context().Done()
						canceled = true
						return nil, fmt.Errorf("blocked request: %w", req.Context().Err())
					}),
				}
				h := tc.newHeartbeat(t, "https://heartbeat.example")
				h.SetHTTPClient(client)
				// Bound the test even if the heartbeat stops applying its own timeout.
				ctx, cancel := context.WithTimeout(t.Context(), tc.defaultTimeout+time.Minute)
				defer cancel()
				start := time.Now()
				require.False(t, h.Ping(ctx, pp.NewSilent(), heartbeat.NewMessage()))
				synctest.Wait()
				require.True(t, canceled)
				// This transport returns as soon as cancellation arrives, so virtual time
				// measures the configured deadline without network or retry-backoff delays.
				require.Equal(t, tc.defaultTimeout, time.Since(start))
			})
		})
	}
}
