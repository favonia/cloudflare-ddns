// Package main probes negative Authorization cases against Cloudflare's
// /user/tokens/verify endpoint and compares the observed responses against
// built-in expected values. It is intended for GitHub Actions and reports
// API behavior drift through stderr, workflow error annotations, and
// GITHUB_STEP_SUMMARY.
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
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

// --- config types ---

type config struct {
	Name             string
	SnapshotDate     string
	URL              string
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

type verifyResponse struct {
	Success  bool             `json:"success"`
	Result   *verifyResult    `json:"result"`
	Errors   []apiError       `json:"errors"`
	Messages []apiMessageInfo `json:"messages"`
}

type verifyResult struct {
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

	pause, err := time.ParseDuration(cfg.PauseBetweenRuns)
	if err != nil {
		return fmt.Errorf("invalid pause_between_runs %q: %w", cfg.PauseBetweenRuns, err)
	}
	timeout, err := time.ParseDuration(cfg.RequestTimeout)
	if err != nil {
		return fmt.Errorf("invalid request_timeout %q: %w", cfg.RequestTimeout, err)
	}

	var drifts []string
	var lines []string

	for _, entry := range cfg.Probes {
		rawDrifts, rawLines := runRawProbe(cfg, entry, timeout)
		drifts = append(drifts, rawDrifts...)
		lines = append(lines, rawLines...)
		sleep(pause)

		if entry.ExpectedSDK == nil {
			continue
		}
		sdkDrifts, sdkLines := runSDKProbe(entry, timeout)
		drifts = append(drifts, sdkDrifts...)
		lines = append(lines, sdkLines...)
		sleep(pause)
	}

	if len(drifts) > 0 {
		writeSummary(buildDriftSummary(cfg, lines, drifts))
		message := fmt.Sprintf(
			"%s API behavior drifted:\n%s",
			cfg.Name, strings.Join(drifts, "\n"),
		)
		fmt.Fprintln(os.Stderr, message)
		return errors.New(message)
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
	flags := flag.NewFlagSet("cloudflare-token-verify-watch", flag.ContinueOnError)
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

func runRawProbe(cfg config, entry probe, timeout time.Duration) ([]string, []string) {
	fmt.Fprintf(os.Stderr, "Probing raw %s...\n", entry.Name)
	raw, err := probeRaw(cfg.URL, cfg.UserAgent, entry, timeout)
	if err != nil {
		line := fmt.Sprintf(
			"- Raw %s [%s]: transport error: %v",
			entry.Name, entry.Kind, err,
		)
		return []string{line}, []string{line}
	}

	rawExpected := formatExpectedRaw(entry.ExpectedRaw)
	rawObserved := formatObservedRaw(raw)
	fmt.Fprintf(os.Stderr, "  observed: %s\n", rawObserved)

	var drifts []string
	if rawExpected != rawObserved {
		drifts = append(drifts, fmt.Sprintf(
			"raw %s [%s]: expected {%s}, observed {%s}",
			entry.Name, entry.Kind, rawExpected, rawObserved,
		))
	}
	line := fmt.Sprintf("- Raw %s [%s]: %s", entry.Name, entry.Kind, rawObserved)
	return drifts, []string{line}
}

func runSDKProbe(entry probe, timeout time.Duration) ([]string, []string) {
	fmt.Fprintf(os.Stderr, "Probing cloudflare-go %s...\n", entry.Name)
	sdk, err := probeSDK(entry, timeout)
	if err != nil {
		line := fmt.Sprintf(
			"- cloudflare-go %s [%s]: unexpected: %v",
			entry.Name, entry.Kind, err,
		)
		return []string{line}, []string{line}
	}

	sdkExpected := formatExpectedSDK(*entry.ExpectedSDK)
	sdkObserved := formatObservedSDK(sdk)
	fmt.Fprintf(os.Stderr, "  observed: %s\n", sdkObserved)

	var drifts []string
	if sdkExpected != sdkObserved {
		drifts = append(drifts, fmt.Sprintf(
			"cloudflare-go %s [%s]: expected {%s}, observed {%s}",
			entry.Name, entry.Kind, sdkExpected, sdkObserved,
		))
	}
	line := fmt.Sprintf(
		"- cloudflare-go %s [%s]: %s",
		entry.Name, entry.Kind, sdkObserved,
	)
	return drifts, []string{line}
}

// --- probers ---

func probeRaw(
	verifyURL, userAgent string, entry probe, timeout time.Duration,
) (observedRaw, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, verifyURL, nil)
	if err != nil {
		return observedRaw{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	if entry.IncludeAuthorization == nil || *entry.IncludeAuthorization {
		req.Header.Set("Authorization", "Bearer "+entry.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return observedRaw{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return observedRaw{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var parsed verifyResponse
	if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
		if err := json.Unmarshal(body, &parsed); err != nil {
			return observedRaw{}, fmt.Errorf(
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

	_, verifyErr := client.VerifyAPIToken(ctx)
	if verifyErr == nil {
		return observedSDK{}, fmt.Errorf(
			"expected VerifyAPIToken to fail for probe %q, but it succeeded",
			entry.Name,
		)
	}

	return observedSDK{
		ErrorType:    classifyErrorType(verifyErr),
		ErrorCodes:   extractErrorCodes(verifyErr),
		ErrorMessage: verifyErr.Error(),
	}, nil
}

func classifyErrorType(err error) string {
	var authorizationError *cloudflare.AuthorizationError
	var requestError *cloudflare.RequestError

	switch {
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
	ErrorCodes() []int
}

func extractErrorCodes(err error) []int {
	var extractor errorCodeExtractor
	if errors.As(err, &extractor) {
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

func buildMatchSummary(cfg config, lines []string) string {
	var builder strings.Builder
	writeHeader(&builder, cfg)
	builder.WriteString("- Status: all probes match the expected API behavior\n")
	builder.WriteString("\n### Observed API behavior\n\n")
	for _, line := range lines {
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func buildDriftSummary(cfg config, lines, drifts []string) string {
	var builder strings.Builder
	writeHeader(&builder, cfg)
	builder.WriteString("- Status: **API behavior drifted**\n")
	builder.WriteString("\n### Observed API behavior\n\n")
	for _, line := range lines {
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	builder.WriteString("\n### Drifted probes\n\n")
	for _, drift := range drifts {
		builder.WriteString("- ")
		builder.WriteString(drift)
		builder.WriteByte('\n')
	}
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

func writeHeader(builder *strings.Builder, cfg config) {
	fmt.Fprintf(builder, "## %s\n\n", cfg.Name)
	fmt.Fprintf(builder, "- Snapshot date: %s\n", cfg.SnapshotDate)
	fmt.Fprintf(builder, "- Endpoint: `%s`\n", cfg.URL)
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
