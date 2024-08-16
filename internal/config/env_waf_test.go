package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest,funlen // paralleltest should not be used because environment vars are global
func TestReadAndAppendWAFListNames(t *testing.T) {
	key := keyPrefix + "WAF_LISTS"

	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      []api.WAFList
		newField      []api.WAFList
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"unset": {
			false, "", nil, nil, true, nil,
		},
		"empty": {
			true, "", nil, nil, true, nil,
		},
		"one": {
			true, "hey/hello",
			nil,
			[]api.WAFList{{AccountID: "hey", ListName: "hello"}},
			true,
			nil,
		},
		"two": {
			true, "hey/hello,here/aloha",
			nil,
			[]api.WAFList{{AccountID: "hey", ListName: "hello"}, {AccountID: "here", ListName: "aloha"}},
			true,
			nil,
		},
		"one+two": {
			true, "hey/hello,here/aloha",
			[]api.WAFList{{AccountID: "there", ListName: "ciao"}},
			[]api.WAFList{
				{AccountID: "there", ListName: "ciao"},
				{AccountID: "hey", ListName: "hello"},
				{AccountID: "here", ListName: "aloha"},
			},
			true,
			nil,
		},
		"invalid-format": {
			true, "+++",
			[]api.WAFList{{AccountID: "there", ListName: "ciao"}},
			[]api.WAFList{{AccountID: "there", ListName: "ciao"}},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, `List %q should be in format "account_id/list_name"`, "+++")
			},
		},
		"invalid-name": {
			true, "hey/hello,+++/!!!,here/aloha",
			[]api.WAFList{{AccountID: "there", ListName: "ciao"}},
			[]api.WAFList{{AccountID: "there", ListName: "ciao"}},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "List name %q contains invalid character %q", "!!!", "!")
			},
		},
		"toolong": {
			true, "x/xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", //nolint:lll
			[]api.WAFList{{AccountID: "there", ListName: "ciao"}},
			[]api.WAFList{{AccountID: "there", ListName: "ciao"}},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "List name \"%.10s...\" is too long (more than 50 characters)", "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx") //nolint:lll
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadAndAppendWAFListNames(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}
