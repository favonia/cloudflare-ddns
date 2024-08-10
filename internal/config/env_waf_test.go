package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

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
		oldField      []string
		newField      []string
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
			true, "hello",
			nil,
			[]string{"hello"},
			true,
			nil,
		},
		"two": {
			true, "hello,aloha",
			nil,
			[]string{"hello", "aloha"},
			true,
			nil,
		},
		"one+two": {
			true, "hello,aloha",
			[]string{"hey"},
			[]string{"hey", "hello", "aloha"},
			true,
			nil,
		},
		"invalid": {
			true, "hello,+++,aloha",
			[]string{"hey"},
			[]string{"hey"},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s=%s contains invalid character %q", key, "+++", "+")
			},
		},
		"toolong": {
			true, "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", //nolint:lll
			[]string{"hey"},
			[]string{"hey"},
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s is too long (more than 50 characters in a name)", key)
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
