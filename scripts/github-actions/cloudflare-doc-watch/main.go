// Package main checks whether selected upstream docs/content still match the
// assumptions recorded in a local JSON config. It is intended for GitHub
// Actions and reports drift through stderr, workflow error annotations, and
// GITHUB_STEP_SUMMARY.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

//nolint:tagliatelle // GitHub-specific
type config struct {
	Name               string                `json:"name"`
	Repo               string                `json:"repo"`
	Ref                string                `json:"ref"`
	Path               string                `json:"path"`
	SnapshotDate       string                `json:"snapshot_date"`
	HistoryURL         string                `json:"history_url"`
	PageURL            string                `json:"page_url"`
	WatchedHeading     string                `json:"watched_heading"`
	LineFilters        []string              `json:"line_filters"`
	ExpectedLines      []string              `json:"expected_lines"`
	WatchedSection     string                `json:"watched_section"`
	ExpectedBullets    []string              `json:"expected_bullets"`
	ExpectedFile       string                `json:"expected_file"`
	WatchLabel         string                `json:"watch_label"`
	Reminders          []string              `json:"reminders"`
	RelatedPaths       []string              `json:"related_paths"`
	JSONRouteSelectors []jsonRouteSelector   `json:"json_route_selectors"`
	JSONPointers       []jsonPointerSelector `json:"json_pointers"`
}

//nolint:tagliatelle // GitHub-specific
type jsonRouteSelector struct {
	Label            string   `json:"label"`
	Name             string   `json:"name"`
	Parent           []string `json:"parent"`
	ExpectedDeeplink string   `json:"expected_deeplink"`
}

type jsonPointerSelector struct {
	Label    string `json:"label"`
	Pointer  string `json:"pointer"`
	Expected any    `json:"expected"`
}

//nolint:tagliatelle // GitHub-specific
type githubCommitResponse []struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message string `json:"message"`
		Author  struct {
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

//nolint:tagliatelle // GitHub-specific
type githubContentsResponse struct {
	Content     string `json:"content"`
	Encoding    string `json:"encoding"`
	DownloadURL string `json:"download_url"`
}

type latestCommit struct {
	SHA     string
	Date    string
	Message string
	URL     string
}

var (
	htmlSectionPattern = regexp.MustCompile(`(?s)%s\s*:\s*<ul>(.*?)</ul>`)
	htmlBulletPattern  = regexp.MustCompile(`(?s)<li>(.*?)</li>`)
	htmlTagPattern     = regexp.MustCompile(`</?(?:code|p|br\s*/?|ul|li)>|<[^>]+>`)
	spacePattern       = regexp.MustCompile(`\s+`)
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: %s <config.json>", os.Args[0])
	}

	configPath := os.Args[1]
	ctx := context.Background()

	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	if cfg.WatchLabel == "" {
		cfg.WatchLabel = "Watched content"
	}

	var latest latestCommit
	if cfg.Repo != "" {
		var err error
		latest, err = fetchLatestCommit(ctx, cfg.Repo, cfg.Ref, cfg.Path)
		if err != nil {
			return err
		}
	}

	expectedItems, actualItems, err := collectWatchItems(ctx, cfg)
	if err != nil {
		return fmt.Errorf("%s\n%s", err, formatCheckContext(cfg, configPath))
	}

	summaryHeaderLines := []string{
		fmt.Sprintf("## %s", cfg.Name),
		"",
	}
	if cfg.Repo != "" {
		summaryHeaderLines = append(summaryHeaderLines,
			fmt.Sprintf("- Source path: `%s/%s` on `%s`", cfg.Repo, cfg.Path, cfg.Ref))
	}
	if cfg.SnapshotDate != "" {
		summaryHeaderLines = append(summaryHeaderLines, fmt.Sprintf("- Expected snapshot date: %s", cfg.SnapshotDate))
	}
	if cfg.PageURL != "" {
		summaryHeaderLines = append(summaryHeaderLines, fmt.Sprintf("- Rendered page: %s", cfg.PageURL))
	}
	if cfg.Repo != "" {
		summaryHeaderLines = append(summaryHeaderLines,
			fmt.Sprintf("- Latest source path commit: [`%s`](%s) on %s", latest.SHA[:12], latest.URL, latest.Date),
			fmt.Sprintf("- Latest source path commit subject: `%s`", latest.Message),
			fmt.Sprintf("- History: %s", cfg.HistoryURL),
		)
	}
	summaryHeader := strings.Join(summaryHeaderLines, "\n") + "\n"

	if slices.Equal(actualItems, expectedItems) {
		writeSummary(summaryHeader + "\n### " + cfg.WatchLabel + "\n\n" + formatBullets(actualItems) + "\n")
		//nolint:forbidigo // this is okay for a small script
		fmt.Printf("%s: watched upstream content still matches the expected assumptions.\n", cfg.Name)
		return nil
	}

	messageLines := []string{
		fmt.Sprintf("Cloudflare doc watch failed for %s.", cfg.Name),
	}
	messageLines = append(messageLines, checkContextLines(cfg, configPath)...)
	if cfg.Repo != "" {
		messageLines = append(messageLines,
			fmt.Sprintf("Source path: %s/%s on %s", cfg.Repo, cfg.Path, cfg.Ref))
	}
	if cfg.SnapshotDate != "" {
		messageLines = append(messageLines, fmt.Sprintf("Expected snapshot date: %s", cfg.SnapshotDate))
	}
	if cfg.Repo != "" {
		messageLines = append(messageLines,
			fmt.Sprintf("Latest source path commit: %s (%s) %s", latest.SHA, latest.Date, latest.Message),
			fmt.Sprintf("Latest source path commit URL: %s", latest.URL),
			fmt.Sprintf("History URL: %s", cfg.HistoryURL),
		)
	}
	if cfg.PageURL != "" {
		messageLines = append(messageLines, fmt.Sprintf("Rendered page URL: %s", cfg.PageURL))
	}
	messageLines = append(messageLines,
		"Expected watched content:",
		formatBullets(expectedItems),
		"Observed watched content:",
		formatBullets(actualItems),
		"Re-check these assumptions:",
		formatBullets(cfg.Reminders),
		"Related repo paths:",
		formatBullets(cfg.RelatedPaths),
	)

	message := strings.Join(messageLines, "\n")
	fmt.Fprintln(os.Stderr, message)
	//nolint:forbidigo // this is okay for a small script
	fmt.Printf("::error title=%s changed::%s\n", cfg.Name, message)
	writeSummary(summaryHeader +
		"\n### Expected watched content\n\n" + formatBullets(expectedItems) +
		"\n\n### Observed watched content\n\n" + formatBullets(actualItems) +
		"\n\n### Re-check these assumptions\n\n" + formatBullets(cfg.Reminders) +
		"\n\n### Related repo paths\n\n" + formatBullets(cfg.RelatedPaths) + "\n")
	return errors.New("watch content drifted")
}

func formatCheckContext(cfg config, configPath string) string {
	return strings.Join(checkContextLines(cfg, configPath), "\n")
}

func checkContextLines(cfg config, configPath string) []string {
	lines := []string{
		fmt.Sprintf("Config: %s", configPath),
	}
	if cfg.PageURL != "" {
		lines = append(lines, fmt.Sprintf("Check URL: %s", cfg.PageURL))
	}
	if cfg.Repo != "" {
		lines = append(lines, fmt.Sprintf("Check source: %s/%s on %s", cfg.Repo, cfg.Path, cfg.Ref))
	}
	for _, target := range checkTargets(cfg) {
		lines = append(lines, fmt.Sprintf("Check target: %s", target))
	}
	return lines
}

func checkTargets(cfg config) []string {
	switch {
	case len(cfg.JSONPointers) > 0:
		targets := make([]string, 0, len(cfg.JSONPointers))
		for _, selector := range cfg.JSONPointers {
			label := selector.Label
			if label == "" {
				label = selector.Pointer
			}
			targets = append(targets, fmt.Sprintf("JSON pointer %q (%s)", selector.Pointer, label))
		}
		return targets
	case len(cfg.JSONRouteSelectors) > 0:
		targets := make([]string, 0, len(cfg.JSONRouteSelectors))
		for _, selector := range cfg.JSONRouteSelectors {
			label := selector.Label
			if label == "" {
				label = selector.Name
			}
			parentLabel := strings.Join(selector.Parent, " / ")
			if parentLabel == "" {
				parentLabel = "(root)"
			}
			targets = append(targets, fmt.Sprintf("dashboard route %q under parent %q (%s)", selector.Name, parentLabel, label))
		}
		return targets
	case cfg.WatchedHeading != "":
		targets := []string{fmt.Sprintf("heading %q", cfg.WatchedHeading)}
		for _, line := range cfg.ExpectedLines {
			targets = append(targets, fmt.Sprintf("expected phrase %q", line))
		}
		return targets
	case cfg.WatchedSection != "":
		targets := []string{fmt.Sprintf("HTML section %q", cfg.WatchedSection)}
		for _, bullet := range cfg.ExpectedBullets {
			targets = append(targets, fmt.Sprintf("expected bullet %q", bullet))
		}
		return targets
	case cfg.PageURL != "":
		targets := make([]string, 0, len(cfg.ExpectedLines)+1)
		if cfg.ExpectedFile != "" {
			targets = append(targets, fmt.Sprintf("expected lines from %q", cfg.ExpectedFile))
		}
		for _, line := range cfg.ExpectedLines {
			targets = append(targets, fmt.Sprintf("expected line %q", line))
		}
		return targets
	default:
		return nil
	}
}

func loadConfig(path string) (config, error) {
	var cfg config
	//nolint:gosec // this is intentional
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config %s: %w", path, err)
	}
	return cfg, nil
}

func collectWatchItems(ctx context.Context, cfg config) ([]string, []string, error) {
	switch {
	case len(cfg.JSONPointers) > 0:
		expected := make([]string, 0, len(cfg.JSONPointers))
		for _, selector := range cfg.JSONPointers {
			expectedJSON, err := canonicalJSON(selector.Expected)
			if err != nil {
				return nil, nil, err
			}
			expected = append(expected, fmt.Sprintf("%s: %s", selector.Label, expectedJSON))
		}
		document, err := fetchDocument(ctx, cfg.Repo, cfg.Ref, cfg.Path)
		if err != nil {
			return nil, nil, err
		}
		actual, err := extractJSONPointerItems(document, cfg.JSONPointers)
		return expected, actual, err
	case len(cfg.JSONRouteSelectors) > 0:
		expected := make([]string, 0, len(cfg.JSONRouteSelectors))
		for _, selector := range cfg.JSONRouteSelectors {
			label := selector.Label
			if label == "" {
				label = selector.Name
			}
			expected = append(expected, fmt.Sprintf("%s: %s", label, selector.ExpectedDeeplink))
		}
		document, err := fetchDocument(ctx, cfg.Repo, cfg.Ref, cfg.Path)
		if err != nil {
			return nil, nil, err
		}
		actual, err := extractJSONRouteItems(document, cfg.JSONRouteSelectors)
		return expected, actual, err
	case cfg.PageURL != "":
		expected, err := loadExpectedLines(cfg)
		if err != nil {
			return nil, nil, err
		}
		document, err := httpGetText(ctx, cfg.PageURL)
		if err != nil {
			return nil, nil, err
		}
		if cfg.WatchedHeading == "" {
			// Plain text mode: compare all non-empty trimmed lines.
			actual := extractPlainTextLines(document)
			return expected, actual, nil
		}
		actual, err := extractMarkdownSectionLines(document, cfg.WatchedHeading, cfg.LineFilters)
		return expected, actual, err
	default:
		document, err := fetchDocument(ctx, cfg.Repo, cfg.Ref, cfg.Path)
		if err != nil {
			return nil, nil, err
		}
		actual, err := extractWatchedBullets(document, cfg.WatchedSection)
		return cfg.ExpectedBullets, actual, err
	}
}

func githubGetJSON(ctx context.Context, requestURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create GitHub API request for %s: %w", requestURL, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cloudflare-ddns-doc-watch")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28") //nolint:canonicalheader // GitHub-specific
	if token := firstNonEmpty(os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API request failed for %s: %w", requestURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort close after fully reading the body.

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read GitHub API response for %s: %w", requestURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API request failed for %s: HTTP %d: %s", requestURL, resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to decode GitHub API response for %s: %w", requestURL, err)
	}
	return nil
}

func httpGetText(ctx context.Context, requestURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request for %s: %w", requestURL, err)
	}
	req.Header.Set("User-Agent", "cloudflare-ddns-doc-watch")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed for %s: %w", requestURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort close after fully reading the body.

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read HTTP response for %s: %w", requestURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP request failed for %s: HTTP %d: %s", requestURL, resp.StatusCode, string(body))
	}
	return string(body), nil
}

func fetchLatestCommit(ctx context.Context, repo, ref, path string) (latestCommit, error) {
	query := url.Values{}
	query.Set("sha", ref)
	query.Set("path", path)
	query.Set("per_page", "1")

	var response githubCommitResponse
	requestURL := fmt.Sprintf("https://api.github.com/repos/%s/commits?%s", repo, query.Encode())
	if err := githubGetJSON(ctx, requestURL, &response); err != nil {
		return latestCommit{}, err
	}
	if len(response) == 0 {
		return latestCommit{}, fmt.Errorf("GitHub returned no commits for %s:%s:%s", repo, ref, path)
	}

	first := response[0]
	message := first.Commit.Message
	if newline := strings.IndexByte(message, '\n'); newline >= 0 {
		message = message[:newline]
	}
	return latestCommit{
		SHA:     first.SHA,
		Date:    first.Commit.Author.Date,
		Message: message,
		URL:     first.HTMLURL,
	}, nil
}

func fetchDocument(ctx context.Context, repo, ref, path string) (string, error) {
	requestURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/contents/%s?ref=%s",
		repo,
		path,
		url.QueryEscape(ref),
	)

	var response githubContentsResponse
	if err := githubGetJSON(ctx, requestURL, &response); err != nil {
		return "", err
	}

	if response.Content != "" && response.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(response.Content, "\n", ""))
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 content for %s:%s:%s: %w", repo, ref, path, err)
		}
		return string(decoded), nil
	}

	if response.DownloadURL != "" {
		return httpGetText(ctx, response.DownloadURL)
	}

	return "", fmt.Errorf("GitHub returned unexpected content metadata for %s:%s:%s", repo, ref, path)
}

// loadExpectedLines returns the expected lines from the config. When
// ExpectedFile is set, it reads the file and uses its non-empty trimmed
// lines; otherwise it falls back to the inline ExpectedLines field.
func loadExpectedLines(cfg config) ([]string, error) {
	if cfg.ExpectedFile == "" {
		return cfg.ExpectedLines, nil
	}
	//nolint:gosec // this is intentional
	data, err := os.ReadFile(cfg.ExpectedFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read expected file %s: %w", cfg.ExpectedFile, err)
	}
	return extractPlainTextLines(string(data)), nil
}

// extractPlainTextLines splits a document into non-empty trimmed lines.
func extractPlainTextLines(document string) []string {
	var lines []string
	for _, line := range strings.Split(document, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func extractWatchedBullets(document, watchedSection string) ([]string, error) {
	pattern := regexp.MustCompile(fmt.Sprintf(htmlSectionPattern.String(), regexp.QuoteMeta(watchedSection)))
	match := pattern.FindStringSubmatch(document)
	if match == nil {
		return nil, fmt.Errorf("could not find watched section %q in the upstream document", watchedSection)
	}

	matches := htmlBulletPattern.FindAllStringSubmatch(match[1], -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("could not extract any watched bullets from section %q", watchedSection)
	}

	bullets := make([]string, 0, len(matches))
	for _, entry := range matches {
		bullets = append(bullets, normalizeHTMLText(entry[1]))
	}
	return bullets, nil
}

func normalizeHTMLText(value string) string {
	value = htmlTagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	value = spacePattern.ReplaceAllString(value, " ")
	return strings.TrimSpace(value)
}

func normalizeMarkdownLine(value string) string {
	value = html.UnescapeString(value)
	value = spacePattern.ReplaceAllString(value, " ")
	return strings.TrimSpace(value)
}

func extractMarkdownSectionLines(document, watchedHeading string, lineFilters []string) ([]string, error) {
	lines := strings.Split(document, "\n")
	headingIndex := -1
	for index, line := range lines {
		if strings.TrimSpace(line) == watchedHeading {
			headingIndex = index
			break
		}
	}
	if headingIndex < 0 {
		return nil, fmt.Errorf("could not find watched heading %q in the upstream page", watchedHeading)
	}

	headingLevel := countHeadingLevel(watchedHeading)
	sectionLines := make([]string, 0)
	for _, line := range lines[headingIndex+1:] {
		stripped := strings.TrimSpace(line)
		if isHeadingAtOrAboveLevel(stripped, headingLevel) {
			break
		}
		if stripped != "" {
			sectionLines = append(sectionLines, normalizeMarkdownLine(stripped))
		}
	}
	if len(sectionLines) == 0 {
		return nil, fmt.Errorf("could not extract any lines from section %q", watchedHeading)
	}
	if len(lineFilters) == 0 {
		return sectionLines, nil
	}

	filterRegexes := make([]*regexp.Regexp, 0, len(lineFilters))
	for _, pattern := range lineFilters {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid line filter %q: %w", pattern, err)
		}
		filterRegexes = append(filterRegexes, re)
	}

	filtered := make([]string, 0)
	for _, line := range sectionLines {
		for _, re := range filterRegexes {
			if re.MatchString(line) {
				filtered = append(filtered, line)
				break
			}
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no lines in section %q matched the configured filters", watchedHeading)
	}
	return filtered, nil
}

func countHeadingLevel(heading string) int {
	level := 0
	for _, r := range heading {
		if r != '#' {
			break
		}
		level++
	}
	return level
}

func isHeadingAtOrAboveLevel(line string, level int) bool {
	if level <= 0 || !strings.HasPrefix(line, "#") {
		return false
	}
	hashes := 0
	for _, r := range line {
		if r != '#' {
			break
		}
		hashes++
	}
	return hashes > 0 && hashes <= level && len(line) > hashes && line[hashes] == ' '
}

func extractJSONRouteItems(document string, selectors []jsonRouteSelector) ([]string, error) {
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(document), &parsed); err != nil {
		return nil, fmt.Errorf("expected a JSON array of dashboard route entries: %w", err)
	}

	items := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		label := selector.Label
		if label == "" {
			label = selector.Name
		}

		matches := make([]map[string]any, 0, 1)
		for _, entry := range parsed {
			if fmt.Sprint(entry["name"]) != selector.Name {
				continue
			}
			if selector.Parent != nil {
				parent, ok := entry["parent"].([]any)
				if !ok || !equalStringSlice(parent, selector.Parent) {
					continue
				}
			}
			matches = append(matches, entry)
		}
		if len(matches) != 1 {
			return nil, fmt.Errorf("expected exactly one dashboard route match for %q, found %d", label, len(matches))
		}

		deeplink, ok := matches[0]["deeplink"].(string)
		if !ok {
			return nil, fmt.Errorf("dashboard route %q has no string deeplink", label)
		}
		items = append(items, fmt.Sprintf("%s: %s", label, deeplink))
	}
	return items, nil
}

func equalStringSlice(raw []any, expected []string) bool {
	if len(raw) != len(expected) {
		return false
	}
	for index := range raw {
		value, ok := raw[index].(string)
		if !ok || value != expected[index] {
			return false
		}
	}
	return true
}

func extractJSONPointerItems(document string, selectors []jsonPointerSelector) ([]string, error) {
	var parsed any
	if err := json.Unmarshal([]byte(document), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse JSON document: %w", err)
	}

	items := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		value, err := resolveJSONPointer(parsed, selector.Pointer)
		if err != nil {
			return nil, err
		}
		rendered, err := canonicalJSON(value)
		if err != nil {
			return nil, err
		}
		items = append(items, fmt.Sprintf("%s: %s", selector.Label, rendered))
	}
	return items, nil
}

func resolveJSONPointer(document any, pointer string) (any, error) {
	if pointer == "" {
		return document, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("JSON pointer must start with '/': %q", pointer)
	}

	current := document
	for _, token := range strings.Split(pointer, "/")[1:] {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch node := current.(type) {
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil {
				return nil, fmt.Errorf("expected array index in JSON pointer %q, got %q", pointer, token)
			}
			if index < 0 || index >= len(node) {
				return nil, fmt.Errorf("JSON pointer %q indexed past the end of an array", pointer)
			}
			current = node[index]
		case map[string]any:
			value, ok := node[token]
			if !ok {
				return nil, fmt.Errorf("JSON pointer %q could not find object key %q", pointer, token)
			}
			current = value
		default:
			return nil, fmt.Errorf("JSON pointer %q stepped into a non-container value", pointer)
		}
	}
	return current, nil
}

func canonicalJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to encode JSON value: %w", err)
	}
	return string(data), nil
}

func formatBullets(items []string) string {
	if len(items) == 0 {
		return ""
	}
	var buffer bytes.Buffer
	for _, item := range items {
		buffer.WriteString("- ")
		buffer.WriteString(item)
		buffer.WriteByte('\n')
	}
	return strings.TrimSuffix(buffer.String(), "\n")
}

func writeSummary(text string) {
	summaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryPath == "" {
		return
	}
	file, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // this is intentional
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
