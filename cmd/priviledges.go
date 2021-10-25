package main

import (
	"syscall"

	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// tryRaiseCap will attempt raise the capabilities.
// The newly gained capabilities (if any) will be dropped later by dropCapabilities.
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

func dropSuperuserGroup(ppfmt pp.PP) {
	defaultGID := syscall.Getegid()
	if defaultGID == 0 {
		defaultGID = syscall.Getgid() // real group ID
		if defaultGID == 0 {
			defaultGID = 1000
		}
	}

	gid := defaultGID
	if !config.ReadNonnegInt(ppfmt, "PGID", &gid) {
		gid = defaultGID
	} else if gid == 0 {
		ppfmt.Errorf(pp.EmojiUserError, "PGID cannot be 0. Using %d instead", defaultGID)
		gid = defaultGID
	}

	// trying to raise cap.SETGID
	tryRaiseCap(cap.SETGID)

	if err := syscall.Setgroups([]int{}); err != nil {
		ppfmt.Infof(pp.EmojiBullet, "Failed to erase supplementary GIDs (which might be fine): %v", err)
	}

	if err := syscall.Setresgid(gid, gid, gid); err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to set GID to %d: %v", gid, err)
	}
}

func dropSuperuser(ppfmt pp.PP) {
	defaultUID := syscall.Geteuid()
	if defaultUID == 0 {
		defaultUID = syscall.Getuid()
		if defaultUID == 0 {
			defaultUID = 1000
		}
	}

	uid := defaultUID
	if !config.ReadNonnegInt(ppfmt, "PUID", &uid) {
		uid = defaultUID
	} else if uid == 0 {
		ppfmt.Errorf(pp.EmojiUserError, "PUID cannot be 0. Using %d instead", defaultUID)
		uid = defaultUID
	}

	// trying to raise cap.SETUID
	tryRaiseCap(cap.SETUID)

	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to set UID to %d: %v", uid, err)
	}
}

func dropCapabilities(ppfmt pp.PP) {
	if err := cap.NewSet().SetProc(); err != nil {
		ppfmt.Errorf(pp.EmojiImpossible, "Failed to drop all capabilities: %v", err)
	}
}

// dropPriviledges drops all privileges as much as possible.
func dropPriviledges(ppfmt pp.PP) {
	if ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Infof(pp.EmojiPriviledges, "Dropping privileges . . .")
		ppfmt = ppfmt.IncIndent()
	}

	// group ID
	dropSuperuserGroup(ppfmt)

	// user ID
	dropSuperuser(ppfmt)

	// all remaining capabilities
	dropCapabilities(ppfmt)
}

func printCapabilities(ppfmt pp.PP) {
	now, err := cap.GetPID(0)
	if err != nil {
		ppfmt.Errorf(pp.EmojiImpossible, "Failed to get the current capabilities: %v", err)
	} else {
		diff, err := now.Cf(cap.NewSet())
		if err != nil {
			ppfmt.Errorf(pp.EmojiImpossible, "Failed to compare capabilities: %v", err)
		} else if diff != 0 {
			ppfmt.Errorf(pp.EmojiError, "The program still retains some additional capabilities: %v", now)
		}
	}
}

func printPriviledges(ppfmt pp.PP) {
	ppfmt.Noticef(pp.EmojiPriviledges, "Priviledges after dropping:")
	inner := ppfmt.IncIndent()

	inner.Noticef(pp.EmojiBullet, "Effective UID:      %d", syscall.Geteuid())
	inner.Noticef(pp.EmojiBullet, "Effective GID:      %d", syscall.Getegid())

	switch groups, err := syscall.Getgroups(); {
	case err != nil:
		inner.Errorf(pp.EmojiImpossible, "Supplementary GIDs: (failed to get them)")
	case len(groups) > 0:
		inner.Noticef(pp.EmojiBullet, "Supplementary GIDs: %d", groups)
	default:
		inner.Noticef(pp.EmojiBullet, "Supplementary GIDs: (empty)")
	}

	printCapabilities(inner)
}
