package domain_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:funlen
func TestParseTemplate(t *testing.T) {
	t.Parallel()
	type f = domain.FQDN
	type w = domain.Wildcard
	for name, tc := range map[string]struct {
		tmpl          string
		ok1           bool
		domain        domain.Domain
		ok2           bool
		expected      string
		prepareMockPP func(m *mocks.MockPP)
	}{
		"empty":           {"", true, f(""), true, "", nil},
		"constant":        {`{{ "string" }}`, true, f(""), true, "string", nil},
		"nospace":         {`! {{- "string" -}} !`, true, f(""), true, "!string!", nil},
		"comments":        {`{* *}`, true, f(""), true, "", nil},
		"variables":       {`{{cool := "cool"}} {{len(cool)}}`, true, f(""), true, " 4", nil},
		"concat":          {`{{"cool" + "string"}}`, true, f(""), true, "coolstring", nil},
		"inDomains/true":  {`{{inDomains("a")}}`, true, f("a"), true, "true", nil},
		"inDomains/false": {`{{inDomains("a.a")}}`, true, f("a"), true, "false", nil},
		"inDomains/ill-formed": {
			`{{inDomains(}}`, false, f(""), false, "",
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "Could not parse the template %q: %v", `{{inDomains(}}`, gomock.Any())
			},
		},
		"inDomains/invalid-argument": {
			`{{inDomains(123)}}`, true, f(""), false, "",
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Errorf(pp.EmojiUserError, "Value %v is not a string", gomock.Any()),
					m.EXPECT().Errorf(pp.EmojiUserError, "Could not execute the template %q: %v", `{{inDomains(123)}}`, gomock.Any()),
				)
			},
		},
		"hasSuffix/true":  {`{{hasSuffix("a")}}`, true, f("a.a"), true, "true", nil},
		"hasSuffix/false": {`{{hasSuffix("a.a")}}`, true, f("a"), true, "false", nil},
		"hasSuffix/invalid-argument": {
			`{{hasSuffix(123)}}`, true, f(""), false, "",
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Errorf(pp.EmojiUserError, "Value %v is not a string", gomock.Any()),
					m.EXPECT().Errorf(pp.EmojiUserError, "Could not execute the template %q: %v", `{{hasSuffix(123)}}`, gomock.Any()),
				)
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			parsed, ok1 := domain.ParseTemplate(mockPP, tc.tmpl)
			require.Equal(t, ok1, tc.ok1)
			if ok1 {
				result, ok2 := parsed(tc.domain)
				require.Equal(t, ok2, tc.ok2)
				if ok2 {
					require.Equal(t, result, tc.expected)
				}
			}
		})
	}
}
