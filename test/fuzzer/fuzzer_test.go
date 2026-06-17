// Package fuzzer_test implements the fuzzing interface for OSS-Fuzz.
package fuzzer_test

import (
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/domainentry"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type ErrorMatcher struct {
	Error error
}

func (m ErrorMatcher) Matches(x any) bool {
	err, ok := x.(error)
	return ok && errors.Is(err, m.Error)
}

func (m ErrorMatcher) String() string {
	return m.Error.Error()
}

const key string = "KEY"

// FuzzParseEntries fuzz tests [domainentry.Parse].
func FuzzParseEntries(f *testing.F) {
	for _, seed := range []string{
		"",
		"example.org",
		"example.org{hostid6=::1,}",
		"example.org{hostid6=[preserve,::1,mac(00-11-22-33-44-55),],}",
		"example.org{hostid6=[::1,,::2]}",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(_ *testing.T, input string) {
		_, _, _ = domainentry.Parse(input)
	})
}

// FuzzParseList fuzz tests [domainexp.ParseList].
func FuzzParseList(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		mockCtrl := gomock.NewController(t)
		mockPP := mocks.NewMockPP(mockCtrl)
		mockPP.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is malformed: %v", key, input, ErrorMatcher{domainexp.ErrUTF8}).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q when "," is expected`,
			key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(
			pp.EmojiUserWarning,
			"%s (%s) contains missing commas; this is accepted for now but will be rejected in version 2.0.0",
			key,
			gomock.Any(),
		).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError,
			"The %s domain in %s (%q) is %q, but it is malformed: %v",
			gomock.Any(), key, input, gomock.Any(), gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError,
			`The %s domain in %s (%q) is %q, but it does not appear to be fully qualified; a fully qualified domain name (FQDN) would look like "*.example.org" or "sub.example.org"`,
			gomock.Any(), key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(
			pp.EmojiUserWarning,
			"%s (%s) contains extra commas; this is accepted for now but will be rejected in version 2.0.0",
			key,
			gomock.Any(),
		).AnyTimes()

		_, _ = domainexp.ParseList(mockPP, key, input)
	})
}

// FuzzParseExpression fuzz tests [domainexp.ParseExpression].
func FuzzParseExpression(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		mockCtrl := gomock.NewController(t)
		mockPP := mocks.NewMockPP(mockCtrl)
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is malformed: %v`, key, input, domainexp.ErrSingleAnd).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is malformed: %v`, key, input, domainexp.ErrSingleOr).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is malformed: %v", key, input, ErrorMatcher{domainexp.ErrUTF8}).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q when %q is expected`, key, input, gomock.Any(), gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is missing %q at the end`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is not a boolean expression: got unexpected token %q`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is not a boolean expression`, key, input).AnyTimes()
		mockPP.EXPECT().Noticef(
			pp.EmojiUserWarning,
			"%s (%s) contains missing commas inside is(...) or sub(...); this is accepted for now but will be rejected in version 2.0.0",
			key,
			gomock.Any(),
		).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError,
			"The %s domain in %s (%q) is %q, but it is malformed: %v",
			gomock.Any(), key, input, gomock.Any(), gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError,
			`The %s domain in %s (%q) is %q, but it does not appear to be fully qualified; a fully qualified domain name (FQDN) would look like "*.example.org" or "sub.example.org"`,
			gomock.Any(), key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(
			pp.EmojiUserWarning,
			"%s (%s) contains extra commas inside is(...) or sub(...); this is accepted for now but will be rejected in version 2.0.0",
			key,
			gomock.Any(),
		).AnyTimes()
		mockPP.EXPECT().Noticef(
			pp.EmojiUserWarning,
			`%s (%q) uses %s() with an empty domain list, which always evaluates to false`,
			key,
			input,
			gomock.Any(),
		).AnyTimes()
		mockPP.EXPECT().Noticef(
			pp.EmojiUserWarning,
			`%s (%q) uses %s with empty domain lists, which always evaluate to false`,
			key,
			input,
			gomock.Any(),
		).AnyTimes()

		_, _ = domainexp.ParseExpression(mockPP, key, input)
	})
}
