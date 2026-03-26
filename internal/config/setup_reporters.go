package config

import (
	"net/url"
	"os"
	"strings"

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

func shoutrrrURLLines() []shoutrrrURLLine {
	raw := os.Getenv("SHOUTRRR")
	if raw == "" {
		return nil
	}

	lines := make([]shoutrrrURLLine, 0)
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, shoutrrrURLLine{lineNum: i + 1, rawURL: line})
	}
	return lines
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
func parseShoutrrrURLs(ppfmt pp.PP) ([]string, bool) {
	lines := shoutrrrURLLines()
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
				"The %s non-empty line of SHOUTRRR contains spaces, "+
					"which suggests that multiple URLs were folded onto one line",
				pp.Ordinal(line.lineNum))
			ppfmt.Infof(pp.EmojiHint,
				"If you meant multiple URLs, put each URL on its own line; if this is one URL, percent-encode spaces")
			ppfmt.Infof(pp.EmojiHint,
				"If you are using YAML folded block style >, use literal block style | instead")
			return nil, false
		}
	}

	// Delay the warning until after the scan so mixed inputs emit only the
	// hard-error path when any line is unsafe.
	if sawWarning {
		for _, line := range lines {
			if classifyShoutrrrURLSpace(line.rawURL) == shoutrrrSpaceWarn {
				ppfmt.Noticef(pp.EmojiUserWarning,
					"The %s non-empty line of SHOUTRRR contains spaces",
					pp.Ordinal(line.lineNum))
			}
		}
		ppfmt.Infof(pp.EmojiHint, "Percent-encode spaces to suppress this warning")
	}

	return urls, true
}

// SetupReporters reads and constructs the configured heartbeat and notifier
// services used by the updater process.
//
// This is a bootstrap path parallel to [RawConfig.ReadEnv] and
// [RawConfig.BuildConfig], not part of them. Its job is limited to the
// reporter-specific environment variables HEALTHCHECKS, UPTIMEKUMA, and
// SHOUTRRR.
//
// Omitting any of these settings is semantically equivalent to setting that
// variable to the empty string.
func SetupReporters(ppfmt pp.PP) (heartbeat.Heartbeat, notifier.Notifier, bool) {
	emptyHeartbeat := heartbeat.NewComposed()
	emptyNotifier := notifier.NewComposed()
	hb := emptyHeartbeat
	nt := emptyNotifier

	if healthchecksURL := Getenv("HEALTHCHECKS"); healthchecksURL != "" {
		h, ok := heartbeat.NewHealthchecks(ppfmt, healthchecksURL)
		if !ok {
			return emptyHeartbeat, emptyNotifier, false
		}
		hb = heartbeat.NewComposed(hb, h)
	}

	if uptimeKumaURL := Getenv("UPTIMEKUMA"); uptimeKumaURL != "" {
		h, ok := heartbeat.NewUptimeKuma(ppfmt, uptimeKumaURL)
		if !ok {
			return emptyHeartbeat, emptyNotifier, false
		}
		hb = heartbeat.NewComposed(hb, h)
	}

	shoutrrrURLs, ok := parseShoutrrrURLs(ppfmt)
	if !ok {
		return emptyHeartbeat, emptyNotifier, false
	}

	if len(shoutrrrURLs) > 0 {
		ppfmt.InfoOncef(pp.MessageExperimentalShoutrrr, pp.EmojiHint,
			"You are using the experimental shoutrrr support available since version 1.12.0")

		s, senderOK := notifier.NewShoutrrr(ppfmt, shoutrrrURLs)
		if !senderOK {
			return emptyHeartbeat, emptyNotifier, false
		}
		nt = notifier.NewComposed(nt, s)
	}

	return hb, nt, true
}
