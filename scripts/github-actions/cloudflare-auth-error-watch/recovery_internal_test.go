package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// These fixtures exercise the raw probe's recovery and final-response contract,
// without depending on Cloudflare availability or sending invalid credentials.
func TestRawProbeRecoversFromRateLimit(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer fixture" {
			t.Errorf("unexpected request method or authorization")
		}
		if calls.Add(1) == 1 {
			writer.Header().Set("Retry-After", "0")
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(writer, "rate limited")
			return
		}
		writer.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(writer,
			`{"success":false,"errors":[{"code":9109,"message":"Invalid access token"}],"messages":[]}`)
	}))
	defer server.Close()
	entry := defaultConfig().Probes[0]
	entry.Token = "fixture"
	got, err := probeRaw(server.URL, "fixture-agent", entry, time.Second)
	if err != nil {
		t.Fatalf("probeRaw: %v", err)
	}
	if got.StatusCode != http.StatusForbidden || len(got.Errors) != 1 || got.Errors[0].Code != 9109 {
		t.Fatalf("final observation = %+v, want HTTP403/code9109", got)
	}
	if calls.Load() != 2 {
		t.Fatalf("requests = %d, want 2", calls.Load())
	}
}

func TestRawProbeRateLimitHasFiniteAttempts(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		writer.Header().Set("Retry-After", "0")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = fmt.Fprint(writer, "rate limited")
	}))
	defer server.Close()
	got, err := probeRaw(server.URL, "fixture-agent", defaultConfig().Probes[0], time.Second)
	if got.StatusCode != http.StatusTooManyRequests || err == nil {
		t.Fatalf("observation = %+v, error = %v; want retained HTTP429 and failed observation", got, err)
	}
	if calls.Load() != 4 {
		t.Fatalf("requests = %d, want initial plus three retries", calls.Load())
	}
}

func TestRawProbeRetryAfterCannotOutliveDeadline(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		writer.Header().Set("Retry-After", "60")
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	_, err := probeRaw(server.URL, "fixture-agent", defaultConfig().Probes[0], 100*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v; want deadline expiry", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("requests = %d; retried before Retry-After", calls.Load())
	}
}

func TestFailureSummaryDoesNotClaimAuthDrift(t *testing.T) {
	t.Parallel()
	summary := buildFailureSummary(defaultConfig(), []diagnostic{{Label: "fixture", Detail: "observation", Expected: ""}},
		[]diagnostic{{Label: "fixture", Detail: "execution failure", Expected: ""}})
	if strings.Contains(summary, "API behavior drifted") {
		t.Fatal("summary claims API drift for an execution failure")
	}
}

func TestRawProbePreservesFinalMismatch(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			writer.Header().Set("Retry-After", "0")
			writer.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(writer,
			`{"success":false,"errors":[{"code":10000,"message":"Authentication error"}],"messages":[]}`)
	}))
	defer server.Close()
	cfg := defaultConfig()
	entry := cfg.Probes[0]
	entry.Endpoint = server.URL
	failures, _ := runRawProbe(cfg, entry, time.Second)
	if len(failures) != 1 {
		t.Fatalf("failures = %v; want final response mismatch", failures)
	}
	if !strings.Contains(failures[0].Detail, "HTTP 401") || !strings.Contains(failures[0].Detail, "code:10000") {
		t.Fatalf("final mismatch missing: %v", failures)
	}
	if calls.Load() != 2 {
		t.Fatalf("requests = %d, want 2", calls.Load())
	}
}

func TestRawProbeFailureReportsReceivedStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(writer, "not JSON")
	}))
	defer server.Close()
	cfg := defaultConfig()
	entry := cfg.Probes[0]
	entry.Endpoint = server.URL
	failures, lines := runRawProbe(cfg, entry, time.Second)
	if len(failures) != 1 || len(lines) != 1 {
		t.Fatalf("failures=%v lines=%v", failures, lines)
	}
	if !strings.Contains(failures[0].Detail, "403") || !strings.Contains(lines[0].Detail, "403") {
		t.Fatalf("received HTTP status lost: failures=%v lines=%v", failures, lines)
	}
}

func TestReadRawResponseClosesBodyOnRequestError(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	recorder.WriteHeader(http.StatusTooManyRequests)
	response := recorder.Result()
	body := &trackedBody{ReadCloser: response.Body, closed: false}
	response.Body = body
	got, err := readRawResponse(response, context.DeadlineExceeded)
	if !errors.Is(err, context.DeadlineExceeded) || got.StatusCode != http.StatusTooManyRequests || !body.closed {
		t.Fatalf("observation=%+v error=%v body closed=%t", got, err, body.closed)
	}
}

type trackedBody struct {
	io.ReadCloser

	closed bool
}

func (body *trackedBody) Close() error {
	body.closed = true
	if err := body.ReadCloser.Close(); err != nil {
		return fmt.Errorf("close fixture body: %w", err)
	}
	return nil
}

func TestSummaryQuotesDiagnosticMarkdown(t *testing.T) {
	t.Parallel()
	detail := "result.status=<nil>\n```\n# response text\n<script>"
	for _, summary := range []string{
		buildMatchSummary(defaultConfig(), []diagnostic{{Label: "probe", Detail: detail, Expected: ""}}),
		buildFailureSummary(defaultConfig(),
			[]diagnostic{{Label: "probe", Detail: detail, Expected: ""}},
			[]diagnostic{{Label: "probe", Detail: detail, Expected: ""}}),
	} {
		if !strings.Contains(summary, "\n  ````text\n  "+strings.ReplaceAll(detail, "\n", "\n  ")+"\n  ````\n") {
			t.Fatalf("diagnostic is not enclosed in a collision-free code block:\n%s", summary)
		}
	}
}

func TestSummaryUsesInlineCodeForSingleLineObservation(t *testing.T) {
	t.Parallel()
	summary := buildMatchSummary(defaultConfig(),
		[]diagnostic{{Label: "Raw zones [invalid]", Detail: "result.status=<nil>", Expected: ""}})
	if !strings.Contains(summary, "- Raw zones \\[invalid\\]: `result.status=<nil>`") {
		t.Fatalf("probe label and value not rendered separately: %s", summary)
	}
}

func TestInlineDiagnosticDelimiters(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct{ name, value, code string }{
		{"plain", "<nil>", "`<nil>`"},
		{"embedded", "a``b", "```a``b```"},
		{"boundaries", "`value`", "`` `value` ``"},
		{"spaces", " a ", "`  a  `"},
		{"all-spaces", "   ", "`   `"},
		{"control", "\x00", "`\"\\x00\"`"},
		{"empty", "", "`\"\"`"},
		{"escape", "\x1b", "`\"\\x1b\"`"},
		{"delete", "\x7f", "`\"\\x7f\"`"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var builder strings.Builder
			writeDiagnosticValue(&builder, "", "probe [case]", testCase.value)
			if got, want := builder.String(), "- probe \\[case\\]: "+testCase.code+"\n"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestMultilineDiagnosticLineEndings(t *testing.T) {
	t.Parallel()
	for _, ending := range []string{"\n", "\r\n", "\r"} {
		t.Run(fmt.Sprintf("%q", ending), func(t *testing.T) {
			t.Parallel()
			var builder strings.Builder
			writeDiagnosticValue(&builder, "", "probe", "first"+ending+"```"+ending+"# literal")
			want := "- probe:\n\n  ````text\n  first\n  ```\n  # literal\n  ````\n\n"
			if builder.String() != want {
				t.Fatalf("got %q, want %q", builder.String(), want)
			}
		})
	}
}
