package config

import (
	"net/url"
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

// parseShoutrrrURLs parses SHOUTRRR into a list of shoutrrr URLs.
//
// The documented format is newline-separated URLs. A single line containing
// raw ASCII spaces is suspicious: it is preserved only when it still parses as
// one URL and only its first space-separated token is URL-like. The parser
// never rewrites one line into multiple URLs.
func parseShoutrrrURLs(ppfmt pp.PP) ([]string, bool) {
	urls := GetenvAsList("SHOUTRRR", "\n")
	sawWarning := false

	for _, rawURL := range urls {
		switch classifyShoutrrrURLSpace(rawURL) {
		case shoutrrrSpaceClean:
			// No suspicious spaces detected.
		case shoutrrrSpaceWarn:
			sawWarning = true
		case shoutrrrSpaceFail:
			ppfmt.Noticef(pp.EmojiUserError,
				"SHOUTRRR contains space characters that look like multiple URLs were folded onto one line")
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
		ppfmt.Noticef(pp.EmojiUserWarning, "SHOUTRRR contains space characters")
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
			"You are using the experimental shoutrrr support added in version 1.12.0")

		s, senderOK := notifier.NewShoutrrr(ppfmt, shoutrrrURLs)
		if !senderOK {
			return emptyHeartbeat, emptyNotifier, false
		}
		nt = notifier.NewComposed(nt, s)
	}

	return hb, nt, true
}
