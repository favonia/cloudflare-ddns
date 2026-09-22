package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The subprocess isolates realMain's signal registrations and default transport.
// Exercise configuration and routing together: cleanup must delete the eligible
// list directly, without first reconciling its items.
func TestOnceCleanupProcess(t *testing.T) {
	t.Parallel()
	if os.Getenv("DDNS_TEST_ONCE_CLEANUP") == "1" {
		transport := &onceCleanupTransport{requests: nil}
		http.DefaultTransport = transport
		require.Zero(t, realMain())
		require.Equal(t, []string{
			"GET /client/v4/accounts/account456/rules/lists",
			"DELETE /client/v4/accounts/account456/rules/lists/list123",
		}, transport.requests)
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	executable, err := os.Executable()
	require.NoError(t, err)
	command := exec.CommandContext(ctx, executable, "-test.run=^TestOnceCleanupProcess$")
	command.Env = []string{
		"DDNS_TEST_ONCE_CLEANUP=1", "CLOUDFLARE_API_TOKEN=fixture-token",
		"UPDATE_CRON=@once", "DELETE_ON_STOP=true", "IP4_PROVIDER=static.empty",
		"IP6_PROVIDER=static.empty", "WAF_LISTS=account456/list",
		"WAF_LIST_DESCRIPTION=description",
	}
	if directory := os.Getenv("GOCOVERDIR"); testing.CoverMode() != "" && directory != "" {
		// This is a test binary, so forward Go's coverage-directory flag as
		// well as the environment variable used by ordinary covered binaries.
		// Sharing the parent's directory lets go test collect the child data.
		command.Args = append(command.Args, "-test.gocoverdir="+directory)
		command.Env = append(command.Env, "GOCOVERDIR="+directory)
	}
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s", output)
}

type onceCleanupTransport struct{ requests []string }

func (transport *onceCleanupTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.requests = append(transport.requests, request.Method+" "+request.URL.Path)
	var response http.Response
	response.StatusCode = http.StatusOK
	response.Header = make(http.Header)
	response.Header.Set("Content-Type", "application/json")
	response.Request = request
	body := `{"success":true,"result":[]}`
	switch request.Method + " " + request.URL.Path {
	case "GET /client/v4/accounts/account456/rules/lists":
		body = `{"success":true,"result":[{"id":"list123","name":"list","kind":"ip","description":"description","numitems":1}]}`
	case "DELETE /client/v4/accounts/account456/rules/lists/list123":
		body = `{"success":true,"result":{"id":"list123"}}`
	}
	response.Body = io.NopCloser(strings.NewReader(body))
	return &response, nil
}
