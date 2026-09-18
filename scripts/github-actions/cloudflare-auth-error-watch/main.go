// Package main probes the real Cloudflare operations behind the updater's
// authentication hints — the Zone listing (ListZones, behind
// hintRecordPermission) and the WAF list listing (ListLists, behind
// hintWAFListPermission) — with well-formed-but-invalid and malformed tokens,
// and compares the observed responses against built-in expected values. It
// guards the AuthenticationError/AuthorizationError classification those hints
// depend on. It reports mismatches and failed observations through stderr,
// workflow error annotations, and GITHUB_STEP_SUMMARY. A failed watch does not
// by itself establish authentication-contract drift.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/hashicorp/go-retryablehttp"
)

// --- config types ---

type config struct {
	Name             string
	SnapshotDate     string
	UserAgent        string
	PauseBetweenRuns string
	RequestTimeout   string
	Reminders        []string
	RelatedPaths     []string
	Probes           []probe
}

type probe struct {
	Name                 string
	Kind                 string
	Endpoint             string // raw HTTP URL probed for this case
	Operation            string // SDK operation: "ListZones" or "ListLists" (empty = no SDK probe)
	AccountID            string // placeholder account ID for ListLists
	Token                string
	IncludeAuthorization *bool
	ExpectedRaw          expectedRaw
	ExpectedSDK          *expectedSDK
}

type expectedRaw struct {
	StatusCode   int
	Success      bool
	ResultStatus *string
	Errors       []apiError
	Messages     []apiMessageInfo
}

type expectedSDK struct {
	ErrorType    string
	ErrorCode    int
	ErrorMessage string
}

// --- API response types ---

type apiError struct {
	Code       int             `json:"code"`
	Message    string          `json:"message"`
	ErrorChain []apiErrorChain `json:"error_chain,omitempty"` //nolint:tagliatelle // Cloudflare API
}

type apiErrorChain struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type apiMessageInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type apiResponse struct {
	Success  bool             `json:"success"`
	Result   *apiResult       `json:"result"`
	Errors   []apiError       `json:"errors"`
	Messages []apiMessageInfo `json:"messages"`
}

type apiResult struct {
	Status string `json:"status"`
}

// --- observed types ---

type observedRaw struct {
	StatusCode   int
	Success      bool
	ResultStatus *string
	Errors       []apiError
	Messages     []apiMessageInfo
}

type observedSDK struct {
	ErrorType    string
	ErrorCodes   []int
	ErrorMessage string
}

// diagnostic separates a single-line probe label from its literal observation.
// Expected is set only for a mismatch; an empty Expected denotes a plain result.
// Markdown is added only at the summary boundary, never parsed out of log text.
type diagnostic struct {
	Label    string
	Detail   string
	Expected string
}

func (d diagnostic) text() string {
	if d.Expected != "" {
		return fmt.Sprintf("%s: expected {%s}, observed {%s}", d.Label, d.Expected, d.Detail)
	}
	return d.Label + ": " + d.Detail
}

// --- main ---

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "::error::%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		return err
	}

	cfg, err := builtInConfig(opts.RunPattern)
	if err != nil {
		return err
	}

	return runConfig(cfg)
}

// runConfig checks every selected raw and SDK observation independently. Any
// mismatch or failed observation fails the watch without stopping later probes.
func runConfig(cfg config) error {
	pause, err := time.ParseDuration(cfg.PauseBetweenRuns)
	if err != nil {
		return fmt.Errorf("invalid pause_between_runs %q: %w", cfg.PauseBetweenRuns, err)
	}
	timeout, err := time.ParseDuration(cfg.RequestTimeout)
	if err != nil {
		return fmt.Errorf("invalid request_timeout %q: %w", cfg.RequestTimeout, err)
	}

	var failures []diagnostic
	var lines []diagnostic

	for _, entry := range cfg.Probes {
		rawFailures, rawLines := runRawProbe(cfg, entry, timeout)
		failures = append(failures, rawFailures...)
		lines = append(lines, rawLines...)
		sleep(pause)

		if entry.ExpectedSDK == nil {
			continue
		}
		sdkFailures, sdkLines := runSDKProbe(entry, timeout)
		failures = append(failures, sdkFailures...)
		lines = append(lines, sdkLines...)
		sleep(pause)
	}

	if len(failures) > 0 {
		writeSummary(buildFailureSummary(cfg, lines, failures))
		failureText := make([]string, 0, len(failures))
		for _, failure := range failures {
			failureText = append(failureText, failure.text())
		}
		message := fmt.Sprintf(
			"%s failed:\n%s",
			cfg.Name, strings.Join(failureText, "\n"),
		)
		fmt.Fprintln(os.Stderr, message)
		return fmt.Errorf("%s failed", cfg.Name)
	}

	writeSummary(buildMatchSummary(cfg, lines))
	//nolint:forbidigo // intentional status output
	fmt.Printf("%s: all probes match the expected API behavior.\n", cfg.Name)
	return nil
}

type options struct {
	RunPattern string
}

func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("cloudflare-auth-error-watch", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var opts options
	flags.StringVar(&opts.RunPattern, "run", "", "regular expression selecting built-in probes to run")
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf("parse flags: %w", err)
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	return opts, nil
}

func builtInConfig(runPattern string) (config, error) {
	cfg := defaultConfig()
	if runPattern == "" {
		return cfg, nil
	}

	pattern, err := regexp.Compile(runPattern)
	if err != nil {
		return config{}, fmt.Errorf("invalid -run pattern: %w", err)
	}

	selected := make([]probe, 0, len(cfg.Probes))
	for _, entry := range cfg.Probes {
		if pattern.MatchString(entry.Name) {
			selected = append(selected, entry)
		}
	}
	if len(selected) == 0 {
		return config{}, fmt.Errorf("no built-in probes match -run %q", runPattern)
	}
	cfg.Probes = slices.Clone(selected)
	return cfg, nil
}

func runRawProbe(cfg config, entry probe, timeout time.Duration) ([]diagnostic, []diagnostic) {
	fmt.Fprintf(os.Stderr, "Probing raw %s...\n", entry.Name)
	label := fmt.Sprintf("Raw %s [%s]", entry.Name, entry.Kind)
	raw, err := probeRaw(entry.Endpoint, cfg.UserAgent, entry, timeout)
	if err != nil {
		detail := err.Error()
		if raw.StatusCode != 0 {
			detail = fmt.Sprintf("HTTP %d: %s", raw.StatusCode, detail)
		}
		line := diagnostic{Label: label, Detail: detail, Expected: ""}
		return []diagnostic{line}, []diagnostic{line}
	}
	expected := formatExpectedRaw(entry.ExpectedRaw)
	observed := formatObservedRaw(raw)
	fmt.Fprintf(os.Stderr, "  observed: %s\n", observed)
	var failures []diagnostic
	if expected != observed {
		failures = append(failures, diagnostic{Label: label, Detail: observed, Expected: expected})
	}
	return failures, []diagnostic{{Label: label, Detail: observed, Expected: ""}}
}

func runSDKProbe(entry probe, timeout time.Duration) ([]diagnostic, []diagnostic) {
	fmt.Fprintf(os.Stderr, "Probing cloudflare-go %s...\n", entry.Name)
	label := fmt.Sprintf("cloudflare-go %s [%s]", entry.Name, entry.Kind)
	sdk, err := probeSDK(entry, timeout)
	if err != nil {
		line := diagnostic{Label: label, Detail: fmt.Sprintf("unexpected: %v", err), Expected: ""}
		return []diagnostic{line}, []diagnostic{line}
	}
	expected := formatExpectedSDK(*entry.ExpectedSDK)
	observed := formatObservedSDK(sdk)
	fmt.Fprintf(os.Stderr, "  observed: %s\n", observed)
	var failures []diagnostic
	if expected != observed {
		failures = append(failures, diagnostic{Label: label, Detail: observed, Expected: expected})
	}
	return failures, []diagnostic{{Label: label, Detail: observed, Expected: ""}}
}

// --- probers ---

// probeRaw retries transient HTTP failures at most three times within timeout,
// including Retry-After waits. It preserves a received status even when the body
// cannot be read or decoded. A final 429 cannot establish the auth baseline.
func probeRaw(
	verifyURL, userAgent string, entry probe, timeout time.Duration,
) (observedRaw, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, verifyURL, nil)
	if err != nil {
		return observedRaw{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	if entry.IncludeAuthorization == nil || *entry.IncludeAuthorization {
		req.Header.Set("Authorization", "Bearer "+entry.Token)
	}

	client := retryablehttp.NewClient()
	client.HTTPClient = http.DefaultClient
	client.RetryMax = 3
	client.Logger = nil
	// The watch needs the final HTTP response even after retry exhaustion.
	client.ErrorHandler = retryablehttp.PassthroughErrorHandler
	resp, err := client.Do(req)
	return readRawResponse(resp, err)
}

// readRawResponse owns and closes any response body, even alongside a request
// error. With no request error, resp and its body must be nonnil. Status remains
// available when receiving or decoding the body fails.
func readRawResponse(resp *http.Response, requestErr error) (observedRaw, error) {
	var raw observedRaw
	if resp != nil {
		raw.StatusCode = resp.StatusCode
		defer resp.Body.Close() //nolint:errcheck // best-effort close
	}
	if requestErr != nil {
		return raw, fmt.Errorf("request failed: %w", requestErr)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return raw, errors.New("rate limited; authentication response not verified")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return raw, fmt.Errorf("failed to read response body: %w", err)
	}

	var parsed apiResponse
	if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
		if err := json.Unmarshal(body, &parsed); err != nil {
			return raw, fmt.Errorf(
				"failed to decode response body %s: %w",
				string(body), err,
			)
		}
	}

	var resultStatus *string
	if parsed.Result != nil {
		resultStatus = &parsed.Result.Status
	}
	return observedRaw{
		StatusCode:   resp.StatusCode,
		Success:      parsed.Success,
		ResultStatus: resultStatus,
		Errors:       parsed.Errors,
		Messages:     parsed.Messages,
	}, nil
}

func probeSDK(entry probe, timeout time.Duration) (observedSDK, error) {
	client, err := cloudflare.NewWithAPIToken(entry.Token)
	if err != nil {
		return observedSDK{}, fmt.Errorf("failed to create cloudflare-go client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var opErr error
	switch entry.Operation {
	case "ListZones":
		_, opErr = client.ListZones(ctx)
	case "ListLists":
		_, opErr = client.ListLists(ctx, cloudflare.AccountIdentifier(entry.AccountID), cloudflare.ListListsParams{})
	default:
		return observedSDK{}, fmt.Errorf("unknown SDK operation %q for probe %q", entry.Operation, entry.Name)
	}
	if opErr == nil {
		return observedSDK{}, fmt.Errorf(
			"expected %s to fail for probe %q, but it succeeded", entry.Operation, entry.Name)
	}

	return observedSDK{
		ErrorType:    classifyErrorType(opErr),
		ErrorCodes:   extractErrorCodes(opErr),
		ErrorMessage: opErr.Error(),
	}, nil
}

func classifyErrorType(err error) string {
	var authenticationError *cloudflare.AuthenticationError
	var authorizationError *cloudflare.AuthorizationError
	var requestError *cloudflare.RequestError

	switch {
	case errors.As(err, &authenticationError):
		return "AuthenticationError"
	case errors.As(err, &authorizationError):
		return "AuthorizationError"
	case errors.As(err, &requestError):
		return "RequestError"
	default:
		return fmt.Sprintf("%T", err)
	}
}

// errorCodeExtractor is implemented by cloudflare-go's error types.
type errorCodeExtractor interface {
	error
	ErrorCodes() []int
}

func extractErrorCodes(err error) []int {
	if extractor, ok := errors.AsType[errorCodeExtractor](err); ok {
		return extractor.ErrorCodes()
	}
	return nil
}

// --- formatting ---

func formatExpectedRaw(expected expectedRaw) string {
	return fmt.Sprintf(
		"HTTP %d, success=%t, result.status=%s, errors=%s, messages=%s",
		expected.StatusCode, expected.Success,
		formatNullableString(expected.ResultStatus),
		formatAPIErrors(expected.Errors),
		formatAPIMessages(expected.Messages),
	)
}

func formatObservedRaw(observed observedRaw) string {
	return fmt.Sprintf(
		"HTTP %d, success=%t, result.status=%s, errors=%s, messages=%s",
		observed.StatusCode, observed.Success,
		formatNullableString(observed.ResultStatus),
		formatAPIErrors(observed.Errors),
		formatAPIMessages(observed.Messages),
	)
}

func formatExpectedSDK(expected expectedSDK) string {
	return fmt.Sprintf(
		"%s, codes=%v, message=%q",
		expected.ErrorType,
		[]int{expected.ErrorCode},
		expected.ErrorMessage,
	)
}

func formatObservedSDK(observed observedSDK) string {
	return fmt.Sprintf(
		"%s, codes=%v, message=%q",
		observed.ErrorType,
		observed.ErrorCodes,
		observed.ErrorMessage,
	)
}

func formatAPIErrors(errs []apiError) string {
	if len(errs) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(errs))
	for _, entry := range errs {
		part := fmt.Sprintf("{code:%d, message:%q", entry.Code, entry.Message)
		if len(entry.ErrorChain) > 0 {
			chainParts := make([]string, 0, len(entry.ErrorChain))
			for _, chain := range entry.ErrorChain {
				chainParts = append(chainParts,
					fmt.Sprintf("{code:%d, message:%q}", chain.Code, chain.Message),
				)
			}
			part += ", error_chain:[" + strings.Join(chainParts, ", ") + "]"
		}
		part += "}"
		parts = append(parts, part)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatAPIMessages(msgs []apiMessageInfo) string {
	if len(msgs) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(msgs))
	for _, entry := range msgs {
		parts = append(parts, fmt.Sprintf("{code:%d, message:%q}", entry.Code, entry.Message))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatNullableString(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%q", *value)
}

// --- summary builders ---

func buildMatchSummary(cfg config, lines []diagnostic) string {
	var builder strings.Builder
	writeHeader(&builder, cfg)
	builder.WriteString("- Status: all probes match the expected API behavior\n")
	builder.WriteString("\n### Observed results\n\n")
	writeDiagnostics(&builder, lines)
	return builder.String()
}

func buildFailureSummary(cfg config, lines, failures []diagnostic) string {
	var builder strings.Builder
	writeHeader(&builder, cfg)
	builder.WriteString("- Status: **failed**\n")
	builder.WriteString("\n### Observed results\n\n")
	writeDiagnostics(&builder, lines)
	builder.WriteString("\n### Failed checks\n\n")
	writeDiagnostics(&builder, failures)
	if len(cfg.Reminders) > 0 {
		builder.WriteString("\n### Reminders\n\n")
		builder.WriteString(formatBullets(cfg.Reminders))
	}
	if len(cfg.RelatedPaths) > 0 {
		builder.WriteString("\n### Related paths\n\n")
		builder.WriteString(formatBullets(cfg.RelatedPaths))
	}
	return builder.String()
}

func writeDiagnostics(builder *strings.Builder, entries []diagnostic) {
	for _, entry := range entries {
		if entry.Expected == "" {
			writeDiagnosticValue(builder, "", entry.Label, entry.Detail)
			continue
		}
		fmt.Fprintf(builder, "- %s:\n", markdownLabel(entry.Label))
		writeDiagnosticValue(builder, "  ", "Expected", entry.Expected)
		writeDiagnosticValue(builder, "  ", "Observed", entry.Detail)
	}
}

// markdownLabel escapes punctuation in built-in, single-line labels.
func markdownLabel(label string) string {
	var builder strings.Builder
	for _, char := range label {
		if strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", char) {
			builder.WriteByte('\\')
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

// writeDiagnosticValue uses inline code for ordinary single-line values. It
// normalizes line endings and nests multiline blocks under their list item.
// Other ASCII controls are displayed as quoted escapes rather than interpreted.
func writeDiagnosticValue(builder *strings.Builder, indent, label, value string) {
	if value == "" || strings.ContainsFunc(value, func(char rune) bool {
		return (char < ' ' && char != '\n' && char != '\r' && char != '\t') || char == 127
	}) {
		value = strconv.Quote(value)
	}
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
	if strings.Contains(value, "\n") {
		fmt.Fprintf(builder, "%s- %s:\n\n", indent, markdownLabel(label))
		var block strings.Builder
		writeCodeBlock(&block, value)
		for line := range strings.SplitSeq(strings.TrimSuffix(block.String(), "\n"), "\n") {
			fmt.Fprintf(builder, "%s  %s\n", indent, line)
		}
		builder.WriteByte('\n')
		return
	}
	fence := "`"
	for strings.Contains(value, fence) {
		fence += "`"
	}
	if strings.HasPrefix(value, "`") || strings.HasSuffix(value, "`") ||
		(strings.HasPrefix(value, " ") && strings.HasSuffix(value, " ") && strings.Trim(value, " ") != "") {
		value = " " + value + " "
	}
	fmt.Fprintf(builder, "%s- %s: %s%s%s\n", indent, markdownLabel(label), fence, value, fence)
}

// writeCodeBlock quotes diagnostic text literally in Markdown. Its fence is
// longer than any backtick run in the content, so remote text cannot close it.
func writeCodeBlock(builder *strings.Builder, text string) {
	fence := "```"
	for strings.Contains(text, fence) {
		fence += "`"
	}
	fmt.Fprintf(builder, "%stext\n%s", fence, text)
	if !strings.HasSuffix(text, "\n") {
		builder.WriteByte('\n')
	}
	fmt.Fprintf(builder, "%s\n", fence)
}

func writeHeader(builder *strings.Builder, cfg config) {
	fmt.Fprintf(builder, "## %s\n\n", cfg.Name)
	fmt.Fprintf(builder, "- Snapshot date: %s\n", cfg.SnapshotDate)
	endpoints := make([]string, 0, len(cfg.Probes))
	seen := map[string]bool{}
	for _, p := range cfg.Probes {
		if p.Endpoint != "" && !seen[p.Endpoint] {
			seen[p.Endpoint] = true
			endpoints = append(endpoints, p.Endpoint)
		}
	}
	for _, e := range endpoints {
		fmt.Fprintf(builder, "- Endpoint: `%s`\n", e)
	}
}

func formatBullets(items []string) string {
	var buffer bytes.Buffer
	for _, item := range items {
		buffer.WriteString("- ")
		buffer.WriteString(item)
		buffer.WriteByte('\n')
	}
	return buffer.String()
}

// --- utilities ---

func sleep(duration time.Duration) {
	fmt.Fprintf(os.Stderr, "Sleeping %v...\n", duration)
	time.Sleep(duration)
}

func writeSummary(text string) {
	summaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryPath == "" {
		return
	}
	//nolint:gosec // intentional
	file, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open GITHUB_STEP_SUMMARY: %v\n", err)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close GITHUB_STEP_SUMMARY: %v\n", err)
		}
	}()
	if _, err := file.WriteString(text); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write GITHUB_STEP_SUMMARY: %v\n", err)
		return
	}
	if !strings.HasSuffix(text, "\n") {
		_, _ = file.WriteString("\n")
	}
}
