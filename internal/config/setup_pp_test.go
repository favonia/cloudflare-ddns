package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest // environment vars are global
func TestSetupPP(t *testing.T) {
	for name, tc := range map[string]struct {
		valEmoji string
		valQuiet string
		ok       bool
		output   string
	}{
		"empty/empty": {"", "", true, "🌟 info\n🌟 notice\n"},
		"true/true":   {"true", " true", true, "🌟 notice\n"},
		"false/false": {"false", "false", true, "info\nnotice\n"},
		"invalid/invalid": {
			"invalid", "invalid", true,
			`😡 EMOJI ("invalid") is not a boolean: strconv.ParseBool: parsing "invalid": invalid syntax
`,
		},
		"false/invalid": {
			"false", "invalid", true,
			`QUIET ("invalid") is not a boolean: strconv.ParseBool: parsing "invalid": invalid syntax
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, "EMOJI", true, tc.valEmoji)
			set(t, "QUIET", true, tc.valQuiet)

			var buf strings.Builder
			ppfmt, ok := config.SetupPP(&buf)

			switch {
			case ok:
				require.NotZero(t, ppfmt)
				ppfmt.Infof(pp.EmojiStar, "info")
				ppfmt.Noticef(pp.EmojiStar, "notice")
				require.Equal(t, tc.output, buf.String())
			case !ok:
				require.Zero(t, ppfmt)
				require.Equal(t, tc.output, buf.String())
			}
		})
	}
}
