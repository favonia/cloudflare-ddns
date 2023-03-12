package droproot

import (
	"syscall"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// readPUID reads PUID.
func readPUID(ppfmt pp.PP) (int, bool) {
	// Calculate the default user ID if PUID is not set
	uid := syscall.Geteuid() // effective user ID
	if uid == 0 {
		uid = syscall.Getuid() // real user ID
		if uid == 0 {
			uid = 1000 // default, if everything is 0
		}
	}

	// The target user ID, after taking PUID into consideration
	if !config.ReadLinuxID(ppfmt, "PUID", &uid) {
		return 0, false
	}

	return uid, true
}

// readPGID returns PGID.
func readPGID(ppfmt pp.PP) (int, bool) {
	// Calculate the default value of PGID
	gid := syscall.Getegid() // effective group ID
	if gid == 0 {
		gid = syscall.Getgid() // real group ID
		if gid == 0 {
			gid = 1000 // default, if everything is 0 (root)
		}
	}

	if !config.ReadLinuxID(ppfmt, "PGID", &gid) {
		return 0, false
	}

	return gid, true
}
