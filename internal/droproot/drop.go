// Package droproot drops root privileges.
package droproot

import (
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func checkUser(ppfmt pp.PP, uid int) {
	euid := syscall.Geteuid()

	// Check if uid is the effective user ID.
	if euid != uid {
		ppfmt.Noticef(pp.EmojiUserWarning, "Failed to reset user ID to %d; current one: %d", uid, euid)
	}
}

func checkGroups(ppfmt pp.PP, gid int) bool {
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

// setGroups tries to set the group IDs.
//
// We do not call cap.SetGroups because of the following two reasons:
//  1. cap.SetGroups will fail when the capability SETGID cannot be obtained,
//     even if Setgid would have worked.
//  2. We use Setresgid instead of Setgid to set all group IDs at once.
func setGroups(ppfmt pp.PP, gid int) bool {
	// Try to raise cap.SETGID so that we can change our group ID
	tryRaiseCapabilitySETGID()

	// Attempt to erase all supplementary groups and set the group ID
	_ = syscall.Setgroups([]int{})
	_ = syscall.Setresgid(gid, gid, gid)

	// Check whether the setting works
	checkGroups(ppfmt, gid)

	return true
}

// setUser sets the user ID to something non-zero.
//
// We do not call cap.SetUID because of the following two reasons:
//  1. cap.SetUID will fail when the capability SETUID cannot be obtained,
//     even if Setuid would have worked.
//  2. We use Setresuid instead of Setuid to set all user IDs at once.
func setUser(ppfmt pp.PP, uid int) bool {
	// Try to raise cap.SETUID so that we can change our user ID
	tryRaiseCapabilitySETUID()

	// Now, set the user ID
	_ = syscall.Setresuid(uid, uid, uid)
	checkUser(ppfmt, uid)

	return true
}

// DropPrivileges drops all privileges as much as possible.
func DropPrivileges(ppfmt pp.PP) bool {
	if ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Infof(pp.EmojiPrivileges, "Dropping privileges . . .")
		ppfmt = ppfmt.IncIndent()
	}

	uid, ok := readPUID(ppfmt)
	if !ok {
		return false
	}

	gid, ok := readPGID(ppfmt)
	if !ok {
		return false
	}

	// 1. Change the group ID first, because the user ID could have given us the power
	// 2. Change the user ID
	// 3. Drop all capabilities (if supported)
	return setGroups(ppfmt, gid) && setUser(ppfmt, uid) && dropCapabilities(ppfmt)
}
