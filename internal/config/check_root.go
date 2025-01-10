package config

import (
	"syscall"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// CheckRoot checks whether the effective user ID is 0 and whether PUID or PGID is set.
func CheckRoot(ppfmt pp.PP) {
	if syscall.Geteuid() == 0 {
		ppfmt.Noticef(pp.EmojiUserWarning, "You are running this updater as root, which is usually a bad idea")
	}

	useDeprecated := false
	if val := Getenv("PUID"); val != "" {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"PUID=%s is ignored since 1.13.0; use Docker's built-in mechanism to set user ID",
			val)
		useDeprecated = true
	}
	if val := Getenv("PGID"); val != "" {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"PGID=%s is ignored since 1.13.0; use Docker's built-in mechanism to set group ID",
			val)
		useDeprecated = true
	}
	if useDeprecated {
		ppfmt.InfoOncef(pp.MessageUpdateDockerTemplate, pp.EmojiHint, "See %s for the new Docker template", pp.ManualURL)
	}
}
