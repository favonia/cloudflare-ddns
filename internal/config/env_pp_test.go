package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest // environment vars are global
func TestReadEmoji(t *testing.T) {
	key := keyPrefix + "EMOJI"
	for name, tc := range map[string]struct {
		set           bool
		val           string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil":   {false, "", true, nil},
		"empty": {true, " ", true, nil},
		"true": {
			true, " true", true,
			func(m *mocks.MockPP) {
				m.EXPECT().SetEmoji(true)
			},
		},
		"false": {
			true, "    false ", true,
			func(m *mocks.MockPP) {
				m.EXPECT().SetEmoji(false)
			},
		},
		"illform": {
			true, "weird", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, "weird", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			var wrappedPP pp.PP = mockPP

			ok := config.ReadEmoji(key, &wrappedPP)
			require.Equal(t, tc.ok, ok)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadQuiet(t *testing.T) {
	key := keyPrefix + "QUIET"
	for name, tc := range map[string]struct {
		set           bool
		val           string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"nil":   {false, "", true, nil},
		"empty": {true, " ", true, nil},
		"true": {
			true, " true", true,
			func(m *mocks.MockPP) {
				m.EXPECT().SetVerbosity(pp.Notice)
			},
		},
		"false": {
			true, "    false ", true,
			func(m *mocks.MockPP) {
				m.EXPECT().SetVerbosity(pp.Info)
			},
		},
		"illform": {
			true, "weird", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, "weird", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			var wrappedPP pp.PP = mockPP

			ok := config.ReadQuiet(key, &wrappedPP)
			require.Equal(t, tc.ok, ok)
		})
	}
}
