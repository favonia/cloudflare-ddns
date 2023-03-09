// Package droproot drops root privileges.
package droproot

import (
	"strconv"
	"strings"
	"syscall"

	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/favonia/cloudflare-ddns/internal/config"
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

	if err := c.SetProc(); err != nil {
		return
	}
}

// dropSuperuserGroup tries to set the group ID to something non-zero.
func dropSuperuserGroup(ppfmt pp.PP) {
	// Calculate the default group ID if PGID is not set
	defaultGID := syscall.Getegid() // effective group ID
	if defaultGID == 0 {
		defaultGID = syscall.Getgid() // real group ID
		if defaultGID == 0 {
			defaultGID = 1000 // default, if everything is 0 (root)
		}
	}

	// The target group ID, after taking PGID into consideration
	gid := defaultGID
	if !config.ReadNonnegInt(ppfmt, "PGID", &gid) {
		gid = defaultGID
	} else if gid == 0 {
		ppfmt.Errorf(pp.EmojiUserError, "PGID cannot be 0. Using PGID=%d instead", defaultGID)
		gid = defaultGID
	}

	// Try to raise cap.SETGID so that we can change our group ID
	tryRaiseCap(cap.SETGID)

	// First, erase all supplementary groups. We do this first because the primary group
	// could have given us the ability to erase supplementary groups.
	if err := syscall.Setgroups([]int{}); err != nil {
		ppfmt.Infof(pp.EmojiBullet, "Failed to erase supplementary GIDs (which might be fine): %v", err)
	}

	// Now, set the group ID
	if err := syscall.Setresgid(gid, gid, gid); err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to set GID to %d: %v", gid, err)
	}
}

// dropSuperuser sets the user ID to something non-zero.
func dropSuperuser(ppfmt pp.PP) {
	// Calculate the default user ID if PUID is not set
	defaultUID := syscall.Geteuid() // effective user ID
	if defaultUID == 0 {
		defaultUID = syscall.Getuid() // real user ID
		if defaultUID == 0 {
			defaultUID = 1000 // default, if everything is 0
		}
	}

	// The target user ID, after taking PUID into consideration
	uid := defaultUID
	if !config.ReadNonnegInt(ppfmt, "PUID", &uid) {
		uid = defaultUID
	} else if uid == 0 {
		ppfmt.Errorf(pp.EmojiUserError, "PUID cannot be 0. Using PUID=%d instead", defaultUID)
		uid = defaultUID
	}

	// Try to raise cap.SETUID so that we can change our user ID
	tryRaiseCap(cap.SETUID)

	// Now, set the user ID
	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to set UID to %d: %v", uid, err)
	}
}

// dropCapabilities drop all capabilities as the last step.
func dropCapabilities(ppfmt pp.PP) {
	if err := cap.NewSet().SetProc(); err != nil {
		ppfmt.Errorf(pp.EmojiImpossible, "Failed to drop all capabilities: %v", err)
	}
}

// DropPriviledges drops all privileges as much as possible.
func DropPriviledges(ppfmt pp.PP) {
	if ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Infof(pp.EmojiPriviledges, "Dropping privileges . . .")
		ppfmt = ppfmt.IncIndent()
	}

	// handle the group ID first, because the user ID could have given us the power
	dropSuperuserGroup(ppfmt)

	// handle the user ID
	dropSuperuser(ppfmt)

	// try to all remaining capabilities
	dropCapabilities(ppfmt)
}

// printCapabilities prints out all remaining capabilities.
func printCapabilities(ppfmt pp.PP) {
	now := cap.GetProc()
	diff, err := now.Cf(cap.NewSet())
	if err != nil {
		ppfmt.Errorf(pp.EmojiImpossible, "Failed to compare capabilities: %v", err)
	} else if diff != 0 {
		ppfmt.Errorf(pp.EmojiError, "The program still retains some additional capabilities: %v", now)
	}
}

func describeGroups(gids []int) string {
	if len(gids) == 0 {
		return "(none)"
	}

	descriptions := make([]string, 0, len(gids))
	for _, gid := range gids {
		descriptions = append(descriptions, strconv.Itoa(gid))
	}
	return strings.Join(descriptions, " ")
}

// PrintPriviledges prints out all remaining privileges.
func PrintPriviledges(ppfmt pp.PP) {
	ppfmt.Noticef(pp.EmojiPriviledges, "Remaining privileges:")
	inner := ppfmt.IncIndent()

	inner.Noticef(pp.EmojiBullet, "Effective UID:      %d", syscall.Geteuid())
	inner.Noticef(pp.EmojiBullet, "Effective GID:      %d", syscall.Getegid())

	if groups, err := syscall.Getgroups(); err != nil {
		inner.Errorf(pp.EmojiImpossible, "Supplementary GIDs: (failed to get them)")
	} else {
		inner.Noticef(pp.EmojiBullet, "Supplementary GIDs: %s", describeGroups(groups))
	}

	printCapabilities(inner)
}
