// Package fuzzer_test implements the fuzzing interface for OSS-Fuzz.
package fuzzer_test

import (
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

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

// FuzzParseDomainList fuzz test [domainexp.ParseList].
func FuzzParseDomainList(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		mockCtrl := gomock.NewController(t)
		mockPP := mocks.NewMockPP(mockCtrl)
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is ill-formed: %v`, key, input, domainexp.ErrSingleAnd).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is ill-formed: %v`, key, input, domainexp.ErrSingleOr).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, input, ErrorMatcher{domainexp.ErrUTF8}).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) contains an ill-formed domain %q: %v`, key, input, gomock.Any(), gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) contains a domain %q that is probably not fully qualified; a fully qualified domain name (FQDN) would look like "*.example.org" or "sub.example.org"`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is missing a comma "," before %q`, key, input, gomock.Any()).AnyTimes()

		_, _ = domainexp.ParseDomainList(mockPP, key, input)
	})
}

// FuzzParseDomainHostIDList fuzz test [domainexp.ParseDomainHostIDList].
func FuzzParseDomainHostIDList(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		mockCtrl := gomock.NewController(t)
		mockPP := mocks.NewMockPP(mockCtrl)
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is ill-formed: %v`, key, input, domainexp.ErrSingleAnd).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is ill-formed: %v`, key, input, domainexp.ErrSingleOr).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, input, ErrorMatcher{domainexp.ErrUTF8}).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) contains an ill-formed domain %q: %v`, key, input, gomock.Any(), gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) contains a domain %q that is probably not fully qualified; a fully qualified domain name (FQDN) would look like "*.example.org" or "sub.example.org"`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is missing a comma "," before %q`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has unclosed "[" at the end`, key, input).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is missing a domain before the opening bracket %q`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q when %q is expected`, key, input, gomock.Any(), gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q when a host ID is expected`, key, input, gomock.Any()).AnyTimes()

		_, _ = domainexp.ParseDomainHostIDList(mockPP, key, input)
	})
}

// FuzzParseExpression fuzz test [domainexp.ParseExpression].
func FuzzParseExpression(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		mockCtrl := gomock.NewController(t)
		mockPP := mocks.NewMockPP(mockCtrl)
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is ill-formed: %v`, key, input, domainexp.ErrSingleAnd).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is ill-formed: %v`, key, input, domainexp.ErrSingleOr).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, input, ErrorMatcher{domainexp.ErrUTF8}).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q when %q is expected`, key, input, gomock.Any(), gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is missing %q at the end`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is not a boolean expression: got unexpected token %q`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is not a boolean expression`, key, input).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) contains an ill-formed domain %q: %v`, key, input, gomock.Any(), gomock.Any()).AnyTimes()
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `%s (%q) is missing a comma "," before %q`, key, input, gomock.Any()).AnyTimes()

		_, _ = domainexp.ParseExpression(mockPP, key, input)
	})
}
