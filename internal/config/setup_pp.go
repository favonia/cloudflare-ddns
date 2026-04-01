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
//
// Omitting these settings is semantically equivalent to EMOJI=true and
// QUIET=false.
//
// Even when it returns ok=false, it still returns a usable pretty printer that
// reflects the formatting state resolved up to the point of failure. This is an
// intentional deviation from the more common Go convention of returning the
// zero value on failure: the caller still needs a printer for top-level startup
// messages such as the final "Bye!" after reporting the concrete parse error.
func SetupPP(output io.Writer) (pp.PP, bool) {
	emoji, verbosity := true, pp.Verbose

	valEmoji, valQuiet := getenv("EMOJI"), getenv("QUIET")

	if valEmoji != "" {
		b, err := strconv.ParseBool(valEmoji)
		ppfmt := pp.New(output, emoji, verbosity)
		if err != nil {
			ppfmt.Noticef(pp.EmojiUserError, "EMOJI (%q) is not a boolean: %v", valEmoji, err)
			return ppfmt, false
		}
		emoji = b
	}

	if valQuiet != "" {
		b, err := strconv.ParseBool(valQuiet)
		ppfmt := pp.New(output, emoji, verbosity)
		if err != nil {
			ppfmt.Noticef(pp.EmojiUserError, "QUIET (%q) is not a boolean: %v", valQuiet, err)
			return ppfmt, false
		}

		if b {
			verbosity = pp.Quiet
		} else {
			verbosity = pp.Verbose
		}
	}

	return pp.New(output, emoji, verbosity), true
}
