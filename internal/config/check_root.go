package config

import (
	"syscall"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// CheckRoot checks whether the effective user ID is 0 and whether PUID or PGID is set.
func CheckRoot(ppfmt pp.PP) {
	if syscall.Geteuid() == 0 {
		ppfmt.Warningf(pp.EmojiUserWarning, "You are running this tool as root, which is usually a bad idea")
	}

	useDeprecated := false
	if val := Getenv("PUID"); val != "" {
		ppfmt.Warningf(pp.EmojiUserError,
			"PUID=%s is ignored; use Docker's built-in mechanism to set user ID",
			val)
		useDeprecated = true
	}
	if val := Getenv("PGID"); val != "" {
		ppfmt.Warningf(pp.EmojiUserError,
			"PGID=%s is ignored; use Docker's built-in mechanism to set group ID",
			val)
		useDeprecated = true
	}
	if useDeprecated {
		ppfmt.Warningf(pp.EmojiHint,
			"See https://github.com/favonia/cloudflare-ddns for the new Docker template")
	}
}
