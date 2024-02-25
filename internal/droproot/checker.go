package droproot

/*
import (
	"slices"
	"strconv"
	"strings"
	"syscall"

	"kernel.org/pub/linux/libs/security/libcap/cap"

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

func checkCapabilities(ppfmt pp.PP) bool {
	now := cap.GetProc()
	diff, err := now.Cf(cap.NewSet())
	switch {
	case err != nil:
		ppfmt.Errorf(pp.EmojiImpossible, "Failed to check Linux capabilities: %v", err)
		return false
	case diff != 0:
		ppfmt.Noticef(pp.EmojiWarning, "Failed to drop all Linux capabilities; current ones: %v", now)
		return false
	default:
		return true
	}
}
*/