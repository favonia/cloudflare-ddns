package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exitStatusCase is shared by the parent and child. Names are only display labels
// and opaque lookup keys; configuration, assertions, and signal timing live here.
// The child applies env overrides, runs arrange before realMain, and runs verify
// afterward with recorded API requests. Nil arrange and verify hooks add no work.
// The parent uses wait to start and reap the child; nil uses CombinedOutput.
type exitStatusCase struct {
	name     string
	env      map[string]string
	expected int
	arrange  func(*testing.T, *exitStatusTransport)
	verify   func(*testing.T, []string)
	wait     func(*testing.T, *exec.Cmd)
}

func exitStatusCases() []exitStatusCase {
	cleanup := map[string]string{"DELETE_ON_STOP": "true", "IP4_PROVIDER": "static.empty", "IP6_PROVIDER": "static.empty"}
	return []exitStatusCase{
		{
			name: "update-success", env: nil, expected: 0,
			arrange: func(_ *testing.T, transport *exitStatusTransport) { transport.itemsPresent = false },
			verify:  nil, wait: nil,
		},
		{name: "update-noop", env: nil, expected: 0, arrange: nil, verify: nil, wait: nil},
		{name: "update-failure", env: nil, expected: 1, arrange: denyExitStatusAPI, verify: nil, wait: nil},
		{
			name: "detection-failure", env: nil, expected: 1,
			arrange: func(t *testing.T, _ *exitStatusTransport) {
				t.Helper()
				setExitStatusDetector(t, func(w http.ResponseWriter, _ *http.Request) {
					_, err := io.WriteString(w, "not an IP address")
					assert.NoError(t, err)
				})
			},
			verify: requireNoExitStatusRequests, wait: nil,
		},
		{
			name: "detection-signal", env: nil, expected: 1,
			arrange: interruptExitStatusDetection, verify: requireNoExitStatusRequests, wait: nil,
		},
		{
			// Cancel an in-flight scheduled update before its signal check.
			// Stopping without cleanup succeeds even though that update was canceled.
			name: "detection-scheduled-signal", env: map[string]string{"UPDATE_CRON": "@every 1h"}, expected: 0,
			arrange: interruptExitStatusDetection, verify: requireNoExitStatusRequests, wait: nil,
		},
		{name: "cleanup-failure", env: cleanup, expected: 1, arrange: denyExitStatusAPI, verify: nil, wait: nil},
		{
			name: "cleanup-fallback", env: cleanup, expected: 0,
			arrange: func(_ *testing.T, transport *exitStatusTransport) {
				transport.respondAPI = func(r *http.Request) *http.Response {
					if r.Method == http.MethodDelete && r.URL.Path == "/client/v4/accounts/account/rules/lists/list-id" {
						return exitStatusResponse(r, http.StatusForbidden, `{"success":false,"errors":[{"code":10000,"message":"list in use"}]}`)
					}
					return nil
				}
			},
			verify: func(t *testing.T, requests []string) {
				t.Helper()
				require.Contains(t, requests, "DELETE /client/v4/accounts/account/rules/lists/list-id")
				require.Contains(t, requests, "DELETE /client/v4/accounts/account/rules/lists/list-id/items")
				require.Contains(t, requests, "GET /client/v4/accounts/account/rules/lists/bulk_operations/operation")
			},
			wait: nil,
		},
		{
			name:     "stop-after-update-failure",
			env:      map[string]string{"UPDATE_CRON": "@every 1h", "DELETE_ON_STOP": "true", "HEALTHCHECKS": "https://report.example/check"},
			expected: 0, arrange: stopExitStatusAfterReport, verify: requireExitStatusListRequest, wait: nil,
		},
		{
			name:     "stop-cleanup-failure",
			env:      map[string]string{"UPDATE_CRON": "@every 1h", "DELETE_ON_STOP": "true", "HEALTHCHECKS": "https://report.example/check", "UPDATE_ON_START": "false"},
			expected: 1, arrange: denyExitStatusAPI, verify: requireExitStatusListRequest, wait: requireExitAfterCountdown,
		},
		{
			name:     "schedule-failure",
			env:      map[string]string{"UPDATE_CRON": "0 0 31 2 *", "DELETE_ON_STOP": "true", "HEALTHCHECKS": "https://report.example/check", "UPDATE_ON_START": "false"},
			expected: 1, arrange: nil, verify: requireExitStatusListRequest, wait: nil,
		},
	}
}

func setExitStatusDetector(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("IP4_PROVIDER", "url:"+server.URL)
}

func interruptExitStatusDetection(t *testing.T, _ *exitStatusTransport) {
	t.Helper()
	setExitStatusDetector(t, func(_ http.ResponseWriter, r *http.Request) {
		assert.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))
		<-r.Context().Done()
	})
}

func denyExitStatusAPI(_ *testing.T, transport *exitStatusTransport) {
	transport.respondAPI = exitStatusPermissionDenied
}

func exitStatusPermissionDenied(r *http.Request) *http.Response {
	return exitStatusResponse(r, http.StatusForbidden, `{"success":false,"errors":[{"code":10000,"message":"permission denied"}]}`)
}

func stopExitStatusAfterReport(_ *testing.T, transport *exitStatusTransport) {
	// The transport serializes both callbacks, so the first completion report
	// switches API behavior from failed update to successful shutdown cleanup.
	stopping := false
	transport.onReport = func(r *http.Request) error {
		if r.URL.Path != "/check/start" && !stopping {
			stopping = true
			if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
				return fmt.Errorf("signal test process: %w", err)
			}
		}
		return nil
	}
	transport.respondAPI = func(r *http.Request) *http.Response {
		if !stopping {
			return exitStatusPermissionDenied(r)
		}
		return nil
	}
}

func requireNoExitStatusRequests(t *testing.T, requests []string) {
	t.Helper()
	require.Empty(t, requests)
}

func requireExitStatusListRequest(t *testing.T, requests []string) {
	t.Helper()
	// A startup/configuration failure cannot satisfy the expected exit by accident.
	require.Contains(t, requests, "GET /client/v4/accounts/account/rules/lists")
}

// Isolate realMain's signal registrations and HTTP transport in a test process.
// Assertions return normally so the test runner can flush subprocess coverage.
func TestExitStatusProcess(t *testing.T) {
	t.Parallel()
	for _, testCase := range exitStatusCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			executable, err := os.Executable()
			require.NoError(t, err)
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestExitStatusProcessHelper$")
			cmd.Env = []string{
				"DDNS_TEST_EXIT_STATUS=" + testCase.name, "CLOUDFLARE_API_TOKEN=fixture-token", "UPDATE_CRON=@once",
				"IP4_PROVIDER=static:192.0.2.1", "IP6_PROVIDER=none", "WAF_LISTS=account/list",
				"WAF_LIST_DESCRIPTION=description", "DETECTION_TIMEOUT=1s", "UPDATE_TIMEOUT=5s",
			}
			if dir := os.Getenv("GOCOVERDIR"); testing.CoverMode() != "" && dir != "" {
				cmd.Args = append(cmd.Args, "-test.gocoverdir="+dir)
				cmd.Env = append(cmd.Env, "GOCOVERDIR="+dir)
			}
			if testCase.wait != nil {
				testCase.wait(t, cmd)
				return
			}
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", output)
		})
	}
}

// requireExitAfterCountdown waits for the scheduled-idle state before signaling.
// Observing the existing countdown output avoids racing cancellation against the
// update loop's signal check; it also verifies shutdown after reaching that state.
// The caller must bound cmd with a context deadline in case no countdown appears.
func requireExitAfterCountdown(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())

	var output strings.Builder
	signaled := false
	var signalErr error
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		output.WriteString(line + "\n")
		if !signaled && strings.Contains(line, "Checking the IP addresses") {
			signalErr = cmd.Process.Signal(syscall.SIGTERM)
			signaled = true
			if signalErr != nil {
				break
			}
		}
	}
	var killErr error
	if scanner.Err() != nil || signalErr != nil {
		killErr = cmd.Process.Kill()
	}
	err = cmd.Wait()
	if !errors.Is(killErr, os.ErrProcessDone) {
		require.NoError(t, killErr)
	}
	require.NoError(t, scanner.Err())
	require.NoError(t, signalErr)
	require.True(t, signaled, "scheduled countdown was not observed:\n%s%s", output.String(), stderr.String())
	require.NoError(t, err, "%s%s", output.String(), stderr.String())
}

//nolint:paralleltest // The isolated child changes environment, HTTP transport, and signal registrations.
func TestExitStatusProcessHelper(t *testing.T) {
	name := os.Getenv("DDNS_TEST_EXIT_STATUS")
	if name == "" {
		t.Skip("subprocess helper")
	}
	for _, testCase := range exitStatusCases() {
		if testCase.name != name {
			continue
		}
		transport := &exitStatusTransport{
			t: t, mu: sync.Mutex{}, requests: nil, itemsPresent: true, onReport: nil, respondAPI: nil,
		}
		for key, value := range testCase.env {
			t.Setenv(key, value)
		}
		if testCase.arrange != nil {
			testCase.arrange(t, transport)
		}
		http.DefaultTransport = transport
		require.Equal(t, testCase.expected, realMain())
		if testCase.verify != nil {
			testCase.verify(t, transport.requests)
		}
		return
	}
	t.Fatalf("unknown exit-status case %q", name)
}

// Hooks run under mu and must not reenter the transport. onReport handles only
// healthcheck requests; respondAPI may override an API response, returning nil
// to use the successful list fixture. Neither hook receives a case identity.
type exitStatusTransport struct {
	t            *testing.T
	mu           sync.Mutex
	requests     []string
	itemsPresent bool
	onReport     func(*http.Request) error
	respondAPI   func(*http.Request) *http.Response
}

func (transport *exitStatusTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if r.URL.Host == "report.example" {
		if transport.onReport != nil {
			if err := transport.onReport(r); err != nil {
				return nil, err
			}
		}
		return exitStatusResponse(r, http.StatusOK, "OK"), nil
	}
	key := r.Method + " " + r.URL.Path
	transport.requests = append(transport.requests, key)
	if transport.respondAPI != nil {
		if response := transport.respondAPI(r); response != nil {
			return response, nil
		}
	}
	var body string
	switch key {
	case "GET /client/v4/accounts/account/rules/lists":
		body = `{"success":true,"result":[{"id":"list-id","name":"list","kind":"ip","description":"description","numitems":1}]}`
	case "GET /client/v4/accounts/account/rules/lists/list-id/items":
		body = `{"success":true,"result":[],"result_info":{"cursors":{"after":""}}}`
		if transport.itemsPresent {
			body = `{"success":true,"result":[{"id":"item-id","ip":"192.0.2.1","comment":""}],"result_info":{"cursors":{"after":""}}}`
		}
	case "POST /client/v4/accounts/account/rules/lists/list-id/items":
		transport.itemsPresent = true
		body = `{"success":true,"result":{"operation_id":"operation"}}`
	case "DELETE /client/v4/accounts/account/rules/lists/list-id":
		body = `{"success":true,"result":{"id":"list-id"}}`
	case "DELETE /client/v4/accounts/account/rules/lists/list-id/items":
		transport.itemsPresent = false
		body = `{"success":true,"result":{"operation_id":"operation"}}`
	case "GET /client/v4/accounts/account/rules/lists/bulk_operations/operation":
		body = `{"success":true,"result":{"id":"operation","status":"completed"}}`
	default:
		transport.t.Errorf("unexpected request %s", key)
		return nil, fmt.Errorf("%w: unexpected request %s", errors.ErrUnsupported, key)
	}
	return exitStatusResponse(r, http.StatusOK, body), nil
}

func exitStatusResponse(r *http.Request, status int, body string) *http.Response {
	var response http.Response
	response.StatusCode = status
	response.Header = make(http.Header)
	response.Header.Set("Content-Type", "application/json")
	response.Request = r
	response.Body = io.NopCloser(strings.NewReader(body))
	return &response
}
