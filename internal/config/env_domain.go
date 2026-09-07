package config

import (
	"errors"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainentry"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func reportEntryDiagnostic(ppfmt pp.PP, key string, input string, diagnostic domainentry.Diagnostic) bool {
	switch diagnostic.Kind {
	case domainentry.KindExtraComma:
		ppfmt.Noticef(pp.EmojiUserWarning,
			"%s (%s) contains extra commas; this is accepted for now but will be rejected in version 2.0.0",
			key, pp.QuotePreviewOrEmptyLabel(input, pp.AdvisoryPreviewLimit, "empty"))
		return true
	case domainentry.KindMissingComma:
		ppfmt.Noticef(pp.EmojiUserWarning,
			"%s (%s) is missing commas; this is accepted for now but will be rejected in version 2.0.0",
			key, pp.QuotePreviewOrEmptyLabel(input, pp.AdvisoryPreviewLimit, "empty"))
		return true
	case domainentry.KindDomainDotTrimming:
		ppfmt.Noticef(pp.EmojiUserWarning,
			`%s accepts %q for now; use %s instead because version 2.0.0 will reject the current spelling`,
			key, input[diagnostic.Span.Start:diagnostic.Span.End], diagnostic.Effective)
		return true
	case domainentry.KindInvalidDomain:
		if errors.Is(diagnostic.Detail, domain.ErrEmptyInteriorLabel) {
			source := input[diagnostic.Span.Start:diagnostic.Span.End]
			ppfmt.Noticef(pp.EmojiUserError,
				`%s has consecutive dots in %q; replace each run with a single dot`, key, source)
			return false
		}
	case domainentry.KindUnknownDomainField, domainentry.KindInvalidHostID6, domainentry.KindInvalidMAC:
		// These existing structured diagnostics use the generic rendering below.
	}
	ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has %s`, key, input, diagnostic.Description(input))
	return false
}

func reportEntryParseError(ppfmt pp.PP, key string, input string, err *syntax.ParseError) {
	expectedToken, expectedTokenOK := errors.AsType[*syntax.ExpectedTokenError](err)
	missingToken, missingTokenOK := errors.AsType[*syntax.MissingTokenError](err)
	switch {
	case expectedTokenOK:
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q when %q is expected`,
			key, input, expectedToken.Got, expectedToken.Expected)
	case missingTokenOK:
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is missing %q at the end`, key, input, missingToken.Expected)
	case errors.Is(err, syntax.ErrUnexpectedToken):
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q`,
			key, input, input[err.Span.Start:err.Span.End])
	default:
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is malformed: %v", key, input, err.Cause)
	}
}

// readDomains reads an environment variable as structured domain entries.
//
// DOMAINS, IP4_DOMAINS, and IP6_DOMAINS are parsed scope declarations, not
// optional settings with reader-owned defaults: unset or empty input leaves the
// field empty (nil). The combined empty-scope case is diagnosed later when the
// scopes are evaluated together.
func readDomains(ppfmt pp.PP, key string, family *ipnet.Family, field *[]domainentry.Entry) bool {
	input := getenv(key)
	entries, diagnostics, err := domainentry.Parse(input)
	if err != nil {
		reportEntryParseError(ppfmt, key, input, err)
		return false
	}

	ok := true
	for _, diagnostic := range diagnostics {
		ok = reportEntryDiagnostic(ppfmt, key, input, diagnostic) && ok
	}
	if !ok {
		return false
	}

	if family != nil && *family == ipnet.IP4 {
		for _, entry := range entries {
			if len(entry.HostID6Opinions) == 0 {
				continue
			}
			ppfmt.Noticef(pp.EmojiUserError,
				`%s (%q) configures hostid6 for %s, but hostid6 only affects IPv6; `+
					`remove hostid6 from this %s entry, `+
					`or configure the IPv6 entry in DOMAINS or IP6_DOMAINS`,
				key, input[entry.Span.Start:entry.Span.End], entry.Domain.Describe(), key)
			return false
		}
	}

	*field = entries
	return true
}
