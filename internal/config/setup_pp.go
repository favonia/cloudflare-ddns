package config

import (
	"io"
	"strconv"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// SetupPP sets up a new PP according to the values of EMOJI and QUIET.
//
// It owns only output-formatting concerns. Reporter services are configured
// separately by [SetupReporters], and updater settings are read separately into
// [RawConfig].
func SetupPP(output io.Writer) (pp.PP, bool) {
	emoji, verbosity := true, pp.DefaultVerbosity

	valEmoji, valQuiet := Getenv("EMOJI"), Getenv("QUIET")

	if valEmoji != "" {
		b, err := strconv.ParseBool(valEmoji)
		if err != nil {
			pp.New(output, emoji, verbosity).Noticef(pp.EmojiUserError, "EMOJI (%q) is not a boolean: %v", valEmoji, err)
			return nil, false
		}
		emoji = b
	}

	if valQuiet != "" {
		b, err := strconv.ParseBool(valQuiet)
		if err != nil {
			pp.New(output, emoji, verbosity).Noticef(pp.EmojiUserError, "QUIET (%q) is not a boolean: %v", valQuiet, err)
			return nil, false
		}

		if b {
			verbosity = pp.Quiet
		} else {
			verbosity = pp.Verbose
		}
	}

	return pp.New(output, emoji, verbosity), true
}
