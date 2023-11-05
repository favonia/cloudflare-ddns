package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
)

//nolint:paralleltest,funlen // paralleltest should not be used because environment vars are global
func TestReadAndAppendShoutrrrURL(t *testing.T) {
	key := keyPrefix + "SHOUTRRR"

	type not = notifier.Notifier

	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      []not
		newField      func(*testing.T, []not)
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"unset": {
			false, "", nil,
			func(t *testing.T, ns []not) {
				t.Helper()
				require.Nil(t, ns)
			},
			true, nil,
		},
		"empty": {
			true, "", nil,
			func(t *testing.T, ns []not) {
				t.Helper()
				require.Nil(t, ns)
			},
			true, nil,
		},
		"generic": {
			true, "generic+https://example.com/api/v1/postStuff",
			nil,
			func(t *testing.T, ns []not) {
				t.Helper()
				require.Len(t, ns, 1)
				m := ns[0]
				s, ok := m.(*notifier.Shoutrrr)
				require.True(t, ok)
				require.Equal(t, []string{"generic"}, s.ServiceNames)
			},
			true,
			nil,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadAndAppendShoutrrrURL(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			tc.newField(t, field)
		})
	}
}
