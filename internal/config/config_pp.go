package config

import (
	"os"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// SetupPP sets up a new PP according to the values of EMOJI and QUIET.
func SetupPP() (pp.PP, bool) {
	ppfmt := pp.New(os.Stdout)
	if !ReadEmoji("EMOJI", &ppfmt) || !ReadQuiet("QUIET", &ppfmt) {
		return nil, false
	}
	return ppfmt, true
}
