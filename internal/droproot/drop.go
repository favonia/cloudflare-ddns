//go:build !nocapdrop

// Package droproot drops root privileges.
package droproot

import (
	"syscall"

	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// tryRaiseCap attempts to raise the capability val.
//
// The newly gained capability (if any) will be dropped by dropCapabilities later.
// We have this function because, in some strange cases, the user might have the
// capability to raise certain capabilities to (ironically) drop more capabilities.
// In any case, it doesn't hurt to try!
func tryRaiseCap(val cap.Value) {
	c, err := cap.GetPID(0)
	if err != nil {
		return
	}

	if err := c.SetFlag(cap.Effective, true, val); err != nil {
		return
	}

	_ = c.SetProc()
}

// setGroups tries to set the group IDs.
//
// We do not call cap.SetGroups because of the following two reasons:
//  1. cap.SetGroups will fail when the capability SETGID cannot be obtained,
//     even if Setgid would have worked.
//  2. We use Setresgid instead of Setgid to set all group IDs at once.
func setGroups(ppfmt pp.PP, gid int) bool {
	// Try to raise cap.SETGID so that we can change our group ID
	tryRaiseCap(cap.SETGID)

	// Attempt to erase all supplementary groups and set the group ID
	_ = syscall.Setgroups([]int{})
	_ = syscall.Setresgid(gid, gid, gid)

	// Check whether the setting works
	checkGroupIDs(ppfmt, gid)

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
	tryRaiseCap(cap.SETUID)

	// Now, set the user ID
	_ = syscall.Setresuid(uid, uid, uid)
	checkUserID(ppfmt, uid)

	return true
}

// dropCapabilities drop all capabilities as the last step.
func dropCapabilities(ppfmt pp.PP) bool {
	_ = cap.NewSet().SetProc()
	checkCapabilities(ppfmt)

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
	// 3. Drop all capabilities
	return setGroups(ppfmt, gid) && setUser(ppfmt, uid) && dropCapabilities(ppfmt)
}
