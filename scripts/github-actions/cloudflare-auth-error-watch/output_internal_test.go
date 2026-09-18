package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// TestWatchOutputHelper runs the production runner with local HTTP fixtures in
// a subprocess. Isolation lets the real SDK use a redirected default transport
// without changing its construction, retry policy, or error classification.
func TestWatchOutputHelper(t *testing.T) { //nolint:paralleltest // subprocess owns http.DefaultTransport
	scenario := os.Getenv("AUTH_WATCH_FIXTURE")
	if scenario == "" {
		return
	}
	var rawCalls atomic.Int32
	var sdkCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		index := -1
		path := strings.TrimPrefix(request.URL.Path, "/client/v4")
		switch path {
		case "/raw/0":
			index = 0
		case "/raw/1":
			index = 1
		case "/raw/2":
			index = 2
		case "/raw/3":
			index = 3
		case "/zones":
			index = 0
		case "/accounts/0123456789abcdef0123456789abcdef/rules/lists":
			index = 2
		default:
			t.Errorf("unexpected probe path: %s", request.URL.Path)
		}
		if path == "/raw/0" {
			attempt := rawCalls.Add(1)
			if serveMalformedFixture(writer, scenario) {
				return
			}
			if scenario == "raw-limited" || (scenario == "recovered" && attempt == 1) {
				writer.Header().Set("Retry-After", "0")
				writer.WriteHeader(http.StatusTooManyRequests)
				_, _ = fmt.Fprint(writer,
					`{"success":false,"errors":[{"code":10502,"message":"Too many authentication failures"}]}`)
				return
			}
			if scenario == "mixed" {
				index = 2
			}
		}
		if path == "/zones" {
			attempt := sdkCalls.Add(1)
			if scenario == "mixed" || (scenario == "sdk-recovered" && attempt == 1) {
				writer.WriteHeader(http.StatusTooManyRequests)
				return
			}
		}
		switch index {
		case 0:
			writer.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprint(writer,
				`{"success":false,"errors":[{"code":9109,"message":"Invalid access token"}],"messages":[]}`)
		case 1:
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(writer,
				`{"success":false,"errors":[{"code":6003,"message":"Invalid request headers",`+
					`"error_chain":[{"code":6111,"message":"Invalid format for Authorization header"}]}],"messages":[]}`)
		case 2:
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(writer,
				`{"success":false,"errors":[{"code":10000,"message":"Authentication error"}],"messages":[]}`)
		case 3:
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(writer,
				`{"success":false,"errors":[{"code":9106,"message":"Authentication failed (status: 400)"}],"messages":[]}`)
		}
	}))
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	http.DefaultTransport = fixtureTransport{
		target: target, base: http.DefaultTransport, failSDK: scenario == "sdk-network",
	}
	cfg := defaultConfig()
	cfg.PauseBetweenRuns = "0s"
	cfg.RequestTimeout = "1500ms"
	for i := range cfg.Probes {
		cfg.Probes[i].Endpoint = fmt.Sprintf("%s/raw/%d", server.URL, i)
	}
	// This is the same failure envelope as main; production runConfig owns the
	// complete probe sequence, stdout, stderr details, and Actions summary.
	runErr := runConfig(cfg)
	server.Close()
	if t.Failed() {
		os.Exit(2)
	}
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "::error::%v\n", runErr)
		os.Exit(1)
	}
	os.Exit(0)
}

// serveMalformedFixture supplies complete versus truncated non-JSON responses.
func serveMalformedFixture(writer http.ResponseWriter, scenario string) bool {
	if scenario != "raw-malformed" && scenario != "raw-body-failure" {
		return false
	}
	if scenario == "raw-body-failure" {
		writer.Header().Set("Content-Length", "100")
	}
	writer.WriteHeader(http.StatusForbidden)
	_, _ = fmt.Fprint(writer, "not JSON")
	return true
}

type fixtureTransport struct {
	target  *url.URL
	base    http.RoundTripper
	failSDK bool
}

func (f fixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if f.failSDK && req.URL.Path == "/client/v4/zones" {
		return nil, errors.New("fixture network unavailable")
	}
	local := req.Clone(req.Context())
	local.URL.Scheme = f.target.Scheme
	local.URL.Host = f.target.Host
	resp, err := f.base.RoundTrip(local)
	if err != nil {
		return nil, fmt.Errorf("local fixture request: %w", err)
	}
	return resp, nil
}

func TestWatchOutput(t *testing.T) {
	t.Parallel()
	captures := make([]string, 8)

	t.Run("scenarios", func(t *testing.T) {
		t.Parallel()
		for index, testCase := range []struct {
			name    string
			success bool
		}{
			{"all-match", true}, {"recovered", true}, {"sdk-recovered", true}, {"raw-limited", false}, {"mixed", false},
			{"raw-malformed", false}, {"raw-body-failure", false}, {"sdk-network", false},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()
				summaryPath := filepath.Join(t.TempDir(), "summary")
				//nolint:gosec // runs this test binary with a fixed helper selector
				cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestWatchOutputHelper$")
				cmd.Env = append(os.Environ(), "AUTH_WATCH_FIXTURE="+testCase.name, "GITHUB_STEP_SUMMARY="+summaryPath)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				err := cmd.Run()
				if (err == nil) != testCase.success {
					t.Fatalf("exit error = %v; stdout=%s stderr=%s", err, &stdout, &stderr)
				}
				summary, readErr := os.ReadFile(summaryPath) //nolint:gosec // test-owned summary path
				if readErr != nil {
					t.Fatal(readErr)
				}
				if !strings.Contains(string(summary), "waf lists malformed token") {
					t.Fatal("remaining probes were not observed")
				}
				if strings.Contains(stderr.String()+string(summary), "API behavior drifted") {
					t.Fatal("aggregate failure overclaims authentication drift")
				}
				if testCase.name == "mixed" && (!strings.Contains(string(summary), "Expected:") ||
					strings.Count(string(summary), "cloudflare\\-go zones invalid token") != 2) {
					t.Fatal("mixed mismatch and SDK failure not retained")
				}
				var capture strings.Builder
				fmt.Fprintf(&capture, "\n## %s\n\nExit success: `%t`\n\n### stdout\n\n", testCase.name, err == nil)
				writeCodeBlock(&capture, stdout.String())
				capture.WriteString("\n### stderr\n\n")
				writeCodeBlock(&capture, stderr.String())
				capture.WriteString("\n### GITHUB_STEP_SUMMARY preview\n\n")
				capture.WriteString("> " + strings.ReplaceAll(strings.TrimSuffix(string(summary), "\n"), "\n", "\n> ") + "\n")
				captures[index] = capture.String()
			})
		}
	})
	t.Cleanup(func() { writeOutputCapture(t, captures) })
}

func writeOutputCapture(t *testing.T, captures []string) {
	t.Helper()

	if t.Failed() {
		return
	}
	if path := os.Getenv("AUTH_WATCH_CAPTURE"); path != "" {
		//nolint:gosec // explicit opt-in path for the maintainer output capture
		if err := os.WriteFile(path, []byte("# Auth watch output review\n\nLocal fixture runs; no Cloudflare requests.\n\n"+
			"stdout, stderr (encounter order within channel), and GITHUB_STEP_SUMMARY preview follow.\n\n"+
			"No quiet mode, heartbeat, or notifier.\n"+strings.Join(captures, "")), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
