package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

var root = mustProjectRoot()

var (
	headingSlugRE        = regexp.MustCompile(`[^\w\- ]+`)
	markdownInlineLinkRE = regexp.MustCompile(`(?s)\[[^\]]+\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	markdownRefLinkRE    = regexp.MustCompile(`(?s)\[([^\]]+)\]\[([^\]]*)\]`)
	markdownRefDefRE     = regexp.MustCompile(`(?m)^\[([^\]]+)\]:\s*(\S+)`)
	htmlTargetRE         = regexp.MustCompile(`(?is)<(?:a|img|script|source)\b[^>]*\s(?:href|src)=["']([^"']+)["']`)
	htmlIDRE             = regexp.MustCompile(`(?is)\bid=["']([^"']+)["']`)
	repoPathRE           = regexp.MustCompile(`(?:\.github|build|cmd|docs|internal|scripts|test)/[A-Za-z0-9_/\-]*\.[A-Za-z0-9._\-]+`)
	urlRE                = regexp.MustCompile("https?://[^\\s<>)\"'`\\]\\\\]+")
	goCommentRE          = regexp.MustCompile(`(?s)//[^\n]*|/\*.*?\*/`)
	redirectStatusCodes  = map[int]bool{301: true, 302: true, 303: true, 307: true, 308: true}
	headFallbackStatuses = map[int]bool{403: true, 405: true, 429: true, 501: true}
	defaultConfigRelPath = path.Join("scripts", "github-actions", "link-check", "config", "default.json")
	maxRedirectHops      = 10
)

type config struct {
	Shared struct {
		Exclude []string `json:"exclude"`
	} `json:"shared"`
	LocalMarkdown struct {
		Include []string `json:"include"`
	} `json:"local_markdown"`
	RepoPaths struct {
		Include []string `json:"include"`
		Ignore  []string `json:"ignore"`
	} `json:"repo_paths"`
	External struct {
		Include           []string `json:"include"`
		Exclude           []string `json:"exclude"`
		IgnoreURLs        []string `json:"ignore_urls"`
		IgnoreURLPatterns []string `json:"ignore_url_patterns"`
	} `json:"external"`
	ExternalProbe struct {
		TimeoutSeconds          float64  `json:"timeout_seconds"`
		Retries                 int      `json:"retries"`
		MaxWorkers              int      `json:"max_workers"`
		UserAgent               string   `json:"user_agent"`
		NetworkErrorsAreWarning bool     `json:"network_errors_are_warnings"`
		WarningStatuses         []int    `json:"warning_statuses"`
		WarningURLPatterns      []string `json:"warning_url_patterns"`
	} `json:"external_probe"`
}

type linkIssue struct {
	Kind   string
	Path   string
	Detail string
}

func (issue linkIssue) Render() string {
	return fmt.Sprintf("%s: %s: %s", issue.Kind, issue.Path, issue.Detail)
}

type probeResult struct {
	URL               string
	Status            int
	FinalURL          string
	Err               string
	Redirected        bool
	PermanentRedirect bool
}

type compiledPatterns struct {
	regexps []*regexp.Regexp
}

func newCompiledPatterns(patterns []string) compiledPatterns {
	regexps := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		regexps = append(regexps, regexp.MustCompile(globToRegexp(pattern)))
	}
	return compiledPatterns{regexps: regexps}
}

func (patterns compiledPatterns) Match(candidate string) bool {
	for _, compiled := range patterns.regexps {
		if compiled.MatchString(candidate) {
			return true
		}
	}
	return false
}

func globToRegexp(pattern string) string {
	var builder strings.Builder
	builder.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				builder.WriteString(".*")
				i++
				continue
			}
			builder.WriteString(`[^/]*`)
		case '?':
			builder.WriteString(`[^/]`)
		default:
			builder.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	builder.WriteString("$")
	return builder.String()
}

func mustProjectRoot() string {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", ".."))
}

func main() {
	mode := flag.String("mode", "all", "which checks to run: local, external, or all")
	configPath := flag.String("config", filepath.Join(root, defaultConfigRelPath), "path to the JSON configuration file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	files, err := gitTrackedFiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	markdownFiles := filterFiles(files, cfg.LocalMarkdown.Include, cfg.Shared.Exclude)
	textFiles := filterFiles(files, cfg.RepoPaths.Include, cfg.Shared.Exclude)
	externalExclude := append(slices.Clone(cfg.Shared.Exclude), cfg.External.Exclude...)
	externalFiles := filterFiles(files, cfg.External.Include, externalExclude)

	exitCode := 0
	switch *mode {
	case "local", "all":
		localIssues := validateLocalMarkdownLinks(markdownFiles)
		localIssues = append(localIssues, validateRepoPaths(textFiles, cfg.RepoPaths.Ignore)...)
		if len(localIssues) > 0 {
			slices.SortFunc(localIssues, func(a, b linkIssue) int {
				return strings.Compare(a.Render(), b.Render())
			})
			for _, issue := range localIssues {
				fmt.Fprintln(os.Stderr, issue.Render())
			}
			exitCode = 1
		} else {
			fmt.Println("Local link checks passed.")
		}
	}

	switch *mode {
	case "external", "all":
		externalURLs := collectExternalURLs(externalFiles, cfg.External.IgnoreURLs, cfg.External.IgnoreURLPatterns)
		fmt.Printf("Collected %d external URLs.\n", len(externalURLs))
		failures, warnings := runExternalProbe(externalURLs, cfg.ExternalProbe)
		for _, warning := range warnings {
			fmt.Println(formatExternalResult("warning", warning))
		}
		if len(failures) > 0 {
			for _, failure := range failures {
				fmt.Fprintln(os.Stderr, formatExternalResult("failure", failure))
			}
			exitCode = 1
		} else {
			fmt.Println("External link probes passed.")
		}
	}

	if *mode != "local" && *mode != "external" && *mode != "all" {
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

func loadConfig(configPath string) (config, error) {
	var cfg config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", configPath, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", configPath, err)
	}
	return cfg, nil
}

func gitTrackedFiles() ([]string, error) {
	command := exec.Command("git", "ls-files", "-z")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("run git ls-files: %w", err)
	}
	rawPaths := bytes.Split(output, []byte{0})
	files := make([]string, 0, len(rawPaths))
	for _, rawPath := range rawPaths {
		if len(rawPath) == 0 {
			continue
		}
		files = append(files, filepath.ToSlash(string(rawPath)))
	}
	return files, nil
}

func filterFiles(files, includePatterns, excludePatterns []string) []string {
	include := newCompiledPatterns(includePatterns)
	exclude := newCompiledPatterns(excludePatterns)
	selected := make([]string, 0, len(files))
	for _, file := range files {
		if len(includePatterns) > 0 && !include.Match(file) {
			continue
		}
		if len(excludePatterns) > 0 && exclude.Match(file) {
			continue
		}
		selected = append(selected, file)
	}
	return selected
}

func validateLocalMarkdownLinks(files []string) []linkIssue {
	issues := make([]linkIssue, 0)
	idCache := map[string]map[string]bool{}
	for _, file := range files {
		text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			issues = append(issues, linkIssue{Kind: "read-error", Path: file, Detail: err.Error()})
			continue
		}
		for _, target := range collectMarkdownTargets(string(text)) {
			if isExternal(target) || strings.HasPrefix(target, "mailto:") {
				continue
			}
			targetPath, fragment := resolveLocalTarget(file, target)
			if targetPath == "" {
				continue
			}
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(targetPath)))
			if err != nil || info == nil {
				issues = append(issues, linkIssue{
					Kind:   "broken-local-link",
					Path:   file,
					Detail: fmt.Sprintf("%s -> missing %s", target, targetPath),
				})
				continue
			}
			if fragment != "" && isMarkdownFile(targetPath) {
				ids, ok := idCache[targetPath]
				if !ok {
					ids = markdownIDs(targetPath)
					idCache[targetPath] = ids
				}
				if !ids[fragment] {
					issues = append(issues, linkIssue{
						Kind:   "broken-anchor",
						Path:   file,
						Detail: fmt.Sprintf("%s -> missing #%s", target, fragment),
					})
				}
			}
		}
	}
	return issues
}

func validateRepoPaths(files []string, ignoredPaths []string) []linkIssue {
	ignored := map[string]bool{}
	for _, ignoredPath := range ignoredPaths {
		ignored[ignoredPath] = true
	}
	issues := make([]linkIssue, 0)
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			issues = append(issues, linkIssue{Kind: "read-error", Path: file, Detail: err.Error()})
			continue
		}
		for _, block := range extractCommentBlocks(file, string(data)) {
			for _, candidate := range repoPathRE.FindAllString(block, -1) {
				candidate = strings.TrimRight(candidate, ".,:;")
				if ignored[candidate] {
					continue
				}
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate))); err != nil {
					issues = append(issues, linkIssue{
						Kind:   "broken-repo-path",
						Path:   file,
						Detail: candidate,
					})
				}
			}
		}
	}
	return issues
}

func collectExternalURLs(files, ignoredURLs, ignoredPatterns []string) []string {
	ignored := map[string]bool{}
	for _, ignoredURL := range ignoredURLs {
		ignored[ignoredURL] = true
	}
	ignoredURLPatterns := compileRegexps(ignoredPatterns)
	found := map[string]bool{}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			continue
		}
		text := string(data)
		if isMarkdownFile(file) {
			for _, target := range collectMarkdownTargets(text) {
				if !isExternal(target) {
					continue
				}
				if ignored[target] || anyRegexpMatch(ignoredURLPatterns, target) {
					continue
				}
				found[target] = true
			}
		}
		for _, target := range urlRE.FindAllString(text, -1) {
			target = strings.TrimRight(target, ".,:;")
			if ignored[target] || anyRegexpMatch(ignoredURLPatterns, target) {
				continue
			}
			found[target] = true
		}
	}
	urls := make([]string, 0, len(found))
	for target := range found {
		urls = append(urls, target)
	}
	slices.Sort(urls)
	return urls
}

func collectMarkdownTargets(text string) []string {
	targets := make([]string, 0)
	for _, match := range markdownInlineLinkRE.FindAllStringSubmatchIndex(text, -1) {
		if match[0] > 0 && text[match[0]-1] == '!' {
			continue
		}
		targets = append(targets, text[match[2]:match[3]])
	}
	refTargets := map[string]string{}
	for _, match := range markdownRefDefRE.FindAllStringSubmatch(text, -1) {
		refTargets[strings.ToLower(strings.TrimSpace(match[1]))] = match[2]
	}
	for _, match := range markdownRefLinkRE.FindAllStringSubmatchIndex(text, -1) {
		if match[0] > 0 && text[match[0]-1] == '!' {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(text[match[2]:match[3]]))
		ref := strings.ToLower(strings.TrimSpace(text[match[4]:match[5]]))
		if ref == "" {
			ref = label
		}
		target, ok := refTargets[ref]
		if ok {
			targets = append(targets, target)
		}
	}
	for _, match := range htmlTargetRE.FindAllStringSubmatch(text, -1) {
		targets = append(targets, match[1])
	}
	return targets
}

func markdownIDs(relativePath string) map[string]bool {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
	if err != nil {
		return nil
	}
	ids := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if heading == "" {
			continue
		}
		ids[markdownAnchor(heading)] = true
	}
	for _, match := range htmlIDRE.FindAllStringSubmatch(string(data), -1) {
		ids[match[1]] = true
	}
	return ids
}

func markdownAnchor(heading string) string {
	cleaned := strings.ToLower(strings.TrimSpace(heading))
	cleaned = headingSlugRE.ReplaceAllString(cleaned, "")
	cleaned = strings.Join(strings.Fields(cleaned), "-")
	return strings.Trim(cleaned, "-")
}

func resolveLocalTarget(sourceFile, target string) (string, string) {
	if target == "" || strings.HasPrefix(target, "mailto:") {
		return "", ""
	}
	pathPart, fragment, _ := strings.Cut(target, "#")
	var resolved string
	switch {
	case strings.HasPrefix(pathPart, "/"):
		resolved = strings.TrimLeft(pathPart, "/")
	case pathPart == "":
		resolved = sourceFile
	default:
		resolved = path.Clean(path.Join(path.Dir(sourceFile), pathPart))
	}
	return resolved, fragment
}

func extractCommentBlocks(file, text string) []string {
	switch path.Ext(file) {
	case ".go":
		return goCommentRE.FindAllString(text, -1)
	case ".yaml", ".yml":
		blocks := make([]string, 0)
		for _, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimLeft(line, " \t")
			if strings.HasPrefix(trimmed, "#") {
				blocks = append(blocks, trimmed)
			}
		}
		return blocks
	default:
		return nil
	}
}

func isMarkdownFile(file string) bool {
	switch path.Ext(file) {
	case ".md", ".markdown":
		return true
	default:
		return false
	}
}

func isExternal(target string) bool {
	parsed, err := url.Parse(target)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func compileRegexps(patterns []string) []*regexp.Regexp {
	regexps := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		regexps = append(regexps, regexp.MustCompile(pattern))
	}
	return regexps
}

func anyRegexpMatch(regexps []*regexp.Regexp, candidate string) bool {
	for _, compiled := range regexps {
		if compiled.MatchString(candidate) {
			return true
		}
	}
	return false
}

func runExternalProbe(urls []string, cfg struct {
	TimeoutSeconds          float64  `json:"timeout_seconds"`
	Retries                 int      `json:"retries"`
	MaxWorkers              int      `json:"max_workers"`
	UserAgent               string   `json:"user_agent"`
	NetworkErrorsAreWarning bool     `json:"network_errors_are_warnings"`
	WarningStatuses         []int    `json:"warning_statuses"`
	WarningURLPatterns      []string `json:"warning_url_patterns"`
}) ([]probeResult, []probeResult) {
	type jobResult struct {
		result probeResult
	}

	warningStatuses := map[int]bool{}
	for _, status := range cfg.WarningStatuses {
		warningStatuses[status] = true
	}
	warningPatterns := compileRegexps(cfg.WarningURLPatterns)
	workerCount := cfg.MaxWorkers
	if workerCount < 1 {
		workerCount = 1
	}

	jobs := make(chan string)
	results := make(chan probeResult)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for target := range jobs {
				results <- probeExternalURL(target, cfg.TimeoutSeconds, cfg.Retries, cfg.UserAgent)
			}
		}()
	}

	go func() {
		for _, target := range urls {
			jobs <- target
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	failures := make([]probeResult, 0)
	warnings := make([]probeResult, 0)
	for result := range results {
		softWarning := anyRegexpMatch(warningPatterns, result.URL)
		switch {
		case softWarning || (cfg.NetworkErrorsAreWarning && result.Status == 0):
			warnings = append(warnings, result)
		case result.Status == 0:
			failures = append(failures, result)
		case result.Status >= 400 && !warningStatuses[result.Status]:
			failures = append(failures, result)
		case warningStatuses[result.Status] || result.Redirected:
			warnings = append(warnings, result)
		}
	}

	slices.SortFunc(failures, func(a, b probeResult) int {
		return strings.Compare(a.URL, b.URL)
	})
	slices.SortFunc(warnings, func(a, b probeResult) int {
		return strings.Compare(a.URL, b.URL)
	})
	return failures, warnings
}

func probeExternalURL(target string, timeoutSeconds float64, retries int, userAgent string) probeResult {
	var last probeResult
	for attempt := 0; attempt <= retries; attempt++ {
		for _, method := range []string{http.MethodHead, http.MethodGet} {
			result := fetchURL(target, method, timeoutSeconds, userAgent)
			last = result
			if method == http.MethodHead && headFallbackStatuses[result.Status] {
				continue
			}
			if result.Status != 0 || result.Err != "" {
				return result
			}
		}
	}
	return last
}

func fetchURL(target, method string, timeoutSeconds float64, userAgent string) probeResult {
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds * float64(time.Second)),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	currentURL := target
	currentMethod := method
	redirected := false
	permanentRedirect := false

	for range maxRedirectHops {
		request, err := http.NewRequest(currentMethod, currentURL, nil)
		if err != nil {
			return probeResult{URL: target, Err: err.Error()}
		}
		request.Header.Set("User-Agent", userAgent)

		response, err := client.Do(request)
		if err != nil {
			return probeResult{URL: target, Err: classifyRequestError(err)}
		}
		io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		response.Body.Close()

		if redirectStatusCodes[response.StatusCode] {
			location, err := response.Location()
			if err != nil {
				return probeResult{URL: target, Status: response.StatusCode, FinalURL: currentURL, Err: err.Error()}
			}
			redirected = true
			if response.StatusCode == http.StatusMovedPermanently || response.StatusCode == http.StatusPermanentRedirect {
				permanentRedirect = true
			}
			currentURL = location.String()
			if response.StatusCode == http.StatusSeeOther {
				currentMethod = http.MethodGet
			}
			continue
		}

		return probeResult{
			URL:               target,
			Status:            response.StatusCode,
			FinalURL:          currentURL,
			Redirected:        redirected,
			PermanentRedirect: permanentRedirect,
		}
	}

	return probeResult{URL: target, Err: "too many redirects"}
}

func classifyRequestError(err error) string {
	var urlError *url.Error
	if errors.As(err, &urlError) {
		if urlError.Err != nil {
			return urlError.Err.Error()
		}
	}
	return err.Error()
}

func formatExternalResult(prefix string, result probeResult) string {
	statusText := result.Err
	if result.Status != 0 {
		statusText = fmt.Sprintf("%d", result.Status)
	}
	if result.Redirected {
		redirectType := "redirect"
		if result.PermanentRedirect {
			redirectType = "permanent redirect"
		}
		return fmt.Sprintf("%s: %s -> %s (%s; %s)", prefix, result.URL, result.FinalURL, statusText, redirectType)
	}
	if result.FinalURL != "" && result.FinalURL != result.URL {
		return fmt.Sprintf("%s: %s -> %s (%s)", prefix, result.URL, result.FinalURL, statusText)
	}
	return fmt.Sprintf("%s: %s (%s)", prefix, result.URL, statusText)
}
