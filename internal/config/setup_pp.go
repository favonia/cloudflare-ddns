package config

import (
	"io"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// SetupPP sets up a new PP according to the values of EMOJI and QUIET.
func SetupPP(output io.Writer) (pp.PP, bool) {
	ppfmt := pp.New(output)
	if !ReadEmoji("EMOJI", &ppfmt) || !ReadQuiet("QUIET", &ppfmt) {
		return nil, false
	}
	return ppfmt, true
}
