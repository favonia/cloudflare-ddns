package main

import (
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

// Isolate realMain's signal registrations and HTTP transport in a test process.
// Assertions return normally so the test runner can flush subprocess coverage.
func TestExitStatusProcess(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{
		"update-success", "update-noop", "update-failure", "detection-failure", "detection-signal",
		"cleanup-failure", "cleanup-fallback", "stop-after-update-failure", "stop-cleanup-failure", "schedule-failure",
	} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			executable, err := os.Executable()
			require.NoError(t, err)
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestExitStatusProcessHelper$")
			cmd.Env = []string{
				"DDNS_TEST_EXIT_STATUS=" + scenario, "CLOUDFLARE_API_TOKEN=fixture-token", "UPDATE_CRON=@once",
				"IP4_PROVIDER=static:192.0.2.1", "IP6_PROVIDER=none", "WAF_LISTS=account/list",
				"WAF_LIST_DESCRIPTION=description", "DETECTION_TIMEOUT=1s", "UPDATE_TIMEOUT=5s",
			}
			if dir := os.Getenv("GOCOVERDIR"); testing.CoverMode() != "" && dir != "" {
				cmd.Args = append(cmd.Args, "-test.gocoverdir="+dir)
				cmd.Env = append(cmd.Env, "GOCOVERDIR="+dir)
			}
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", output)
		})
	}
}

//nolint:paralleltest // The isolated child changes environment, HTTP transport, and signal registrations.
func TestExitStatusProcessHelper(t *testing.T) {
	scenario := os.Getenv("DDNS_TEST_EXIT_STATUS")
	if scenario == "" {
		t.Skip("subprocess helper")
	}
	var expected int
	transport := &exitStatusTransport{
		t: t, scenario: scenario, mu: sync.Mutex{}, requests: nil,
		itemsPresent: scenario != "update-success", stopping: false,
	}
	http.DefaultTransport = transport
	switch scenario {
	case "update-success", "update-noop":
	case "update-failure", "cleanup-failure", "detection-failure", "detection-signal", "stop-cleanup-failure", "schedule-failure":
		expected = 1
	case "cleanup-fallback", "stop-after-update-failure":
	default:
		t.Fatalf("unknown scenario %s", scenario)
	}
	if strings.HasPrefix(scenario, "cleanup-") {
		t.Setenv("DELETE_ON_STOP", "true")
		t.Setenv("IP4_PROVIDER", "static.empty")
		t.Setenv("IP6_PROVIDER", "static.empty")
	}
	if strings.HasPrefix(scenario, "stop-") || scenario == "schedule-failure" {
		t.Setenv("UPDATE_CRON", "@every 1h")
		t.Setenv("DELETE_ON_STOP", "true")
		t.Setenv("HEALTHCHECKS", "https://report.example/check")
		if scenario != "stop-after-update-failure" {
			t.Setenv("UPDATE_ON_START", "false")
		}
		if scenario == "schedule-failure" {
			t.Setenv("UPDATE_CRON", "0 0 31 2 *")
		}
	}
	if strings.HasPrefix(scenario, "detection-") {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if scenario == "detection-signal" {
				assert.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))
				<-r.Context().Done()
				return
			}
			_, err := io.WriteString(w, "not an IP address")
			assert.NoError(t, err)
		}))
		defer server.Close()
		t.Setenv("IP4_PROVIDER", "url:"+server.URL)
	}
	require.Equal(t, expected, realMain())
	if strings.HasPrefix(scenario, "detection-") {
		require.Empty(t, transport.requests)
	}
	if scenario == "cleanup-fallback" {
		require.Contains(t, transport.requests, "DELETE /client/v4/accounts/account/rules/lists/list-id")
		require.Contains(t, transport.requests, "DELETE /client/v4/accounts/account/rules/lists/list-id/items")
		require.Contains(t, transport.requests, "GET /client/v4/accounts/account/rules/lists/bulk_operations/operation")
	}
	if scenario == "schedule-failure" || strings.HasPrefix(scenario, "stop-") {
		// A startup/configuration failure cannot satisfy the expected exit by accident.
		require.Contains(t, transport.requests, "GET /client/v4/accounts/account/rules/lists")
	}
}

type exitStatusTransport struct {
	t            *testing.T
	scenario     string
	mu           sync.Mutex
	requests     []string
	itemsPresent bool
	stopping     bool
}

func (transport *exitStatusTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if r.URL.Host == "report.example" {
		if strings.HasPrefix(transport.scenario, "stop-") && r.URL.Path != "/check/start" && !transport.stopping {
			transport.stopping = true
			if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
				return nil, fmt.Errorf("signal test process: %w", err)
			}
		}
		return exitStatusResponse(r, http.StatusOK, "OK"), nil
	}
	key := r.Method + " " + r.URL.Path
	transport.requests = append(transport.requests, key)
	denied := transport.scenario == "update-failure" || transport.scenario == "cleanup-failure" || transport.scenario == "stop-cleanup-failure" || (transport.scenario == "stop-after-update-failure" && !transport.stopping)
	if denied {
		return exitStatusResponse(r, http.StatusForbidden, `{"success":false,"errors":[{"code":10000,"message":"permission denied"}]}`), nil
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
		if transport.scenario == "cleanup-fallback" {
			return exitStatusResponse(r, http.StatusForbidden, `{"success":false,"errors":[{"code":10000,"message":"list in use"}]}`), nil
		}
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
