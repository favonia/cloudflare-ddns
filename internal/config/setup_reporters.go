package config

import (
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// isShoutrrrURLLikeToken reports whether a token has a URI scheme.
//
// This check is only for suspicious-space detection. Final URL and service
// validation is delegated to shoutrrr.
func isShoutrrrURLLikeToken(token string) bool {
	parsed, err := url.Parse(token)
	return err == nil && parsed.Scheme != ""
}

type shoutrrrSpaceClassification int

const (
	shoutrrrSpaceClean shoutrrrSpaceClassification = iota
	shoutrrrSpaceWarn
	shoutrrrSpaceFail
)

func classifyShoutrrrURLSpace(rawURL string) shoutrrrSpaceClassification {
	if !strings.Contains(rawURL, " ") {
		return shoutrrrSpaceClean
	}

	wholeLineURLLike := isShoutrrrURLLikeToken(rawURL)
	firstTokenURLLike := false
	urlLikeCount := 0
	isFirstToken := true

	for token := range strings.SplitSeq(rawURL, " ") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		tokenURLLike := isShoutrrrURLLikeToken(token)
		if isFirstToken {
			firstTokenURLLike = tokenURLLike
			isFirstToken = false
		}
		if tokenURLLike {
			urlLikeCount++
		}
	}

	if urlLikeCount == 1 && firstTokenURLLike && wholeLineURLLike {
		return shoutrrrSpaceWarn
	}
	return shoutrrrSpaceFail
}

type shoutrrrURLLine struct {
	lineNum int
	rawURL  string
}

// parseShoutrrrLines splits raw shoutrrr configuration text into non-blank,
// non-comment lines, preserving 1-based line numbers so diagnostics point at the
// true source line. A line is a comment when its first non-whitespace character
// is '#'. Inline '#' is deliberately NOT treated as a comment because shoutrrr
// URLs may contain '#' in a query or fragment; do not route this through
// file.ProcessLines, which strips inline '#'.
func parseShoutrrrLines(raw string) []shoutrrrURLLine {
	lines := make([]shoutrrrURLLine, 0)
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, shoutrrrURLLine{lineNum: i + 1, rawURL: line})
	}
	return lines
}

// shoutrrrSource is one origin of shoutrrr URLs: its raw text plus a
// human-readable name used in diagnostics (e.g. "SHOUTRRR" or
// "the file specified by SHOUTRRR_FILE").
type shoutrrrSource struct {
	name string
	raw  string
}

// parseShoutrrrURLs parses SHOUTRRR into a list of shoutrrr URLs.
//
// The input contract is newline-separated URLs, with each configured line kept
// as exactly one URL. The parser never rewrites one ambiguous line into
// multiple URLs.
//
// This function only handles parsing and validation of the raw SHOUTRRR input.
// Final shoutrrr client construction and message delivery behavior are handled
// downstream.
//
// A single line containing raw ASCII spaces is suspicious because YAML folded
// input or similar formatting mistakes can merge multiple URLs onto one line.
// Such a line is preserved only when the whole line is still URL-like and only
// its first space-separated token is URL-like; any other spaced line fails
// early instead of deferring the ambiguity to downstream shoutrrr behavior.
//
// Warnings are delayed until after the full scan so mixed inputs emit only the
// hard-error path when any line is unsafe. If future heuristics are added here,
// preserve the newline-separated contract unless the public interface is
// intentionally redesigned. Any future notifier input surface should normalize
// back to the same one-URL-per-line contract instead of making parsing behavior
// format-dependent.
func parseShoutrrrURLs(ppfmt pp.PP, src shoutrrrSource) ([]string, bool) {
	lines := parseShoutrrrLines(src.raw)
	urls := make([]string, 0, len(lines))
	sawWarning := false

	for _, line := range lines {
		urls = append(urls, line.rawURL)
		switch classifyShoutrrrURLSpace(line.rawURL) {
		case shoutrrrSpaceClean:
			// No suspicious spaces detected.
		case shoutrrrSpaceWarn:
			sawWarning = true
		case shoutrrrSpaceFail:
			ppfmt.Noticef(pp.EmojiUserError,
				"Line %d of %s contains spaces, "+
					"which suggests that multiple URLs were folded onto one line",
				line.lineNum, src.name)
			ppfmt.Infof(pp.EmojiHint,
				`If you meant multiple URLs, put each URL on its own line; if this is one URL, encode spaces as "%%20"`)
			ppfmt.Infof(pp.EmojiHint,
				`If you use YAML folded block style ">", switch to literal block style "|"`)
			return nil, false
		}
	}

	// Delay the warning until after the scan so mixed inputs emit only the
	// hard-error path when any line is unsafe.
	if sawWarning {
		for _, line := range lines {
			if classifyShoutrrrURLSpace(line.rawURL) == shoutrrrSpaceWarn {
				ppfmt.Noticef(pp.EmojiUserWarning,
					"The %s non-empty line of %s contains spaces",
					pp.Ordinal(line.lineNum), src.name)
			}
		}
		ppfmt.Infof(pp.EmojiHint, `Encode spaces as "%%20" in URLs to suppress this warning`)
	}

	return urls, true
}

// readShoutrrrFileURLs reads and parses SHOUTRRR_FILE, if set. An unset
// SHOUTRRR_FILE yields no URLs and no error. A configured but unreadable or
// non-absolute path is an error. A readable file that parses to no URLs (blank
// or comment-only) yields no URLs and no error.
func readShoutrrrFileURLs(ppfmt pp.PP) ([]string, bool) {
	path := getenv("SHOUTRRR_FILE")
	if path == "" {
		return nil, true
	}

	raw, ok := file.ReadRawString(ppfmt, path)
	if !ok {
		return nil, false
	}

	return parseShoutrrrURLs(ppfmt, shoutrrrSource{name: "the file specified by SHOUTRRR_FILE", raw: raw})
}

// reconcileShoutrrrURLs combines the URL lists from SHOUTRRR and SHOUTRRR_FILE.
// When both sources provide URLs, the two lists must be equal up to ordering,
// counting multiplicity (sorted, not deduplicated). Otherwise the single
// non-empty list is used, or none.
func reconcileShoutrrrURLs(ppfmt pp.PP, envURLs, fileURLs []string) ([]string, bool) {
	switch {
	case len(envURLs) > 0 && len(fileURLs) > 0:
		if !slices.Equal(slices.Sorted(slices.Values(envURLs)), slices.Sorted(slices.Values(fileURLs))) {
			ppfmt.Noticef(pp.EmojiUserError,
				"The URLs in SHOUTRRR and the file specified by SHOUTRRR_FILE differ; they must specify the same URLs")
			return nil, false
		}
		return envURLs, true
	case len(envURLs) > 0:
		return envURLs, true
	default:
		return fileURLs, true
	}
}

// SetupReporters reads and constructs the configured heartbeat and notifier
// services used by the updater process.
//
// This is a bootstrap path parallel to [RawConfig.ReadEnv] and
// [RawConfig.BuildConfig], not part of them. Its job is limited to the
// reporter-specific environment variables HEALTHCHECKS, UPTIMEKUMA, SHOUTRRR,
// and SHOUTRRR_FILE.
//
// Omitting any of these settings is semantically equivalent to setting that
// variable to the empty string.
func SetupReporters(ppfmt pp.PP) (heartbeat.Heartbeat, notifier.Notifier, bool) {
	emptyHeartbeat := heartbeat.NewComposed()
	emptyNotifier := notifier.NewComposed()
	hb := emptyHeartbeat
	nt := emptyNotifier

	if healthchecksURL := getenv("HEALTHCHECKS"); healthchecksURL != "" {
		h, ok := heartbeat.NewHealthchecks(ppfmt, healthchecksURL)
		if !ok {
			return emptyHeartbeat, emptyNotifier, false
		}
		hb = heartbeat.NewComposed(hb, h)
	}

	if uptimeKumaURL := getenv("UPTIMEKUMA"); uptimeKumaURL != "" {
		h, ok := heartbeat.NewUptimeKuma(ppfmt, uptimeKumaURL)
		if !ok {
			return emptyHeartbeat, emptyNotifier, false
		}
		hb = heartbeat.NewComposed(hb, h)
	}

	envShoutrrrURLs, ok := parseShoutrrrURLs(ppfmt, shoutrrrSource{name: "SHOUTRRR", raw: os.Getenv("SHOUTRRR")})
	if !ok {
		return emptyHeartbeat, emptyNotifier, false
	}

	fileShoutrrrURLs, ok := readShoutrrrFileURLs(ppfmt)
	if !ok {
		return emptyHeartbeat, emptyNotifier, false
	}

	shoutrrrURLs, ok := reconcileShoutrrrURLs(ppfmt, envShoutrrrURLs, fileShoutrrrURLs)
	if !ok {
		return emptyHeartbeat, emptyNotifier, false
	}

	if len(shoutrrrURLs) > 0 {
		s, senderOK := notifier.NewShoutrrr(ppfmt, shoutrrrURLs)
		if !senderOK {
			return emptyHeartbeat, emptyNotifier, false
		}
		nt = notifier.NewComposed(nt, s)
	}

	return hb, nt, true
}
