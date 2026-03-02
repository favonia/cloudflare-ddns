package setter_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAcceptsManagedRecordRegexes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		config setterConfig
	}{
		{
			name:   "nil",
			config: setterConfig{}, //nolint:exhaustruct // Zero value means no managed-record filter in this case.
		},
		{
			name: "compiled",
			config: setterConfig{
				managedRecordCommentRegex: regexp.MustCompile(""),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, h := newSetterHarnessWithConfig(t, tc.config)
			require.NotNil(t, h.setter)
			require.Equal(t, tc.config.recordFilter(), h.recordFilter)
		})
	}
}
