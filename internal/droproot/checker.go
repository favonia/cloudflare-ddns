package droproot

import (
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/exp/slices"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func checkUserID(ppfmt pp.PP, uid int) {
	euid := syscall.Geteuid()

	// Check if uid is the effective user ID.
	if euid != uid {
		ppfmt.Noticef(pp.EmojiUserWarning, "Failed to reset user ID to %d; current one: %d", uid, euid)
	}
}

func checkGroupIDs(ppfmt pp.PP, gid int) bool {
	egid := syscall.Getegid()
	groups, err := syscall.Getgroups()
	if err != nil {
		ppfmt.Errorf(pp.EmojiImpossible, "Failed to get supplementary group IDs: %v", err)
		return false
	}

	// Check if gid is the only effective group ID.
	ok := egid == gid && !slices.ContainsFunc(groups, func(g int) bool { return g != gid })
	if !ok {
		descriptions := make([]string, 1, len(groups)+1)
		descriptions[0] = strconv.Itoa(egid)
		for _, g := range groups {
			if g != egid {
				descriptions = append(descriptions, strconv.Itoa(g))
			}
		}
		ppfmt.Warningf(pp.EmojiUserWarning,
			"Failed to reset group IDs to only %d; current ones: %s",
			gid, strings.Join(descriptions, ", "))
	}

	return ok
}
