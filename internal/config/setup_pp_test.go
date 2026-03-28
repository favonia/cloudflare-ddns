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
			"invalid", "invalid", false,
			`😡 EMOJI ("invalid") is not a boolean: strconv.ParseBool: parsing "invalid": invalid syntax
`,
		},
		"false/invalid": {
			"false", "invalid", false,
			`QUIET ("invalid") is not a boolean: strconv.ParseBool: parsing "invalid": invalid syntax
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, "EMOJI", true, tc.valEmoji)
			set(t, "QUIET", true, tc.valQuiet)

			var buf strings.Builder
			ppfmt, ok := config.SetupPP(&buf)
			require.Equal(t, tc.ok, ok)
			require.NotZero(t, ppfmt)

			switch {
			case ok:
				ppfmt.Infof(pp.EmojiStar, "info")
				ppfmt.Noticef(pp.EmojiStar, "notice")
				require.Equal(t, tc.output, buf.String())
			case !ok:
				ppfmt.Infof(pp.EmojiBye, "Bye!")
				if tc.valEmoji == "false" {
					require.Equal(t, tc.output+"Bye!\n", buf.String())
				} else {
					require.Equal(t, tc.output+"👋 Bye!\n", buf.String())
				}
			}
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestSetupPPOmissionMatchesCanonicalExplicitValues(t *testing.T) {
	render := func(t *testing.T, setEmoji bool, emoji string, setQuiet bool, quiet string) string {
		t.Helper()

		set(t, "EMOJI", setEmoji, emoji)
		set(t, "QUIET", setQuiet, quiet)

		var buf strings.Builder
		ppfmt, ok := config.SetupPP(&buf)
		require.True(t, ok)
		ppfmt.Infof(pp.EmojiStar, "info")
		ppfmt.Noticef(pp.EmojiStar, "notice")
		return buf.String()
	}

	implicit := render(t, false, "", false, "")
	explicit := render(t, true, "true", true, "false")

	require.Equal(t, implicit, explicit)
}
