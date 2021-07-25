package main

import (
	"syscall"

	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/favonia/cloudflare-ddns-go/internal/config"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
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

func dropSuperuserGroup(indent pp.Indent) {
	defaultGID := syscall.Getegid()
	if defaultGID == 0 {
		defaultGID = syscall.Getgid() // real group ID
		if defaultGID == 0 {
			defaultGID = 1000
		}
	}

	gid := defaultGID
	if !config.ReadNonnegInt(quiet.QUIET, indent, "PGID", &gid) {
		gid = defaultGID
	} else if gid == 0 {
		pp.Printf(indent, pp.EmojiUserError, "PGID cannot be 0. Using %d instead.", defaultGID)
		gid = defaultGID
	}

	// trying to raise cap.SETGID
	tryRaiseCap(cap.SETGID)

	if err := syscall.Setgroups([]int{}); err != nil {
		pp.Printf(indent, pp.EmojiBullet, "Failed to erase supplementary GIDs (which might be fine): %v", err)
	}

	if err := syscall.Setresgid(gid, gid, gid); err != nil {
		pp.Printf(indent, pp.EmojiUserError, "Failed to set GID to %d: %v", gid, err)
	}
}

func dropSuperuser(indent pp.Indent) {
	defaultUID := syscall.Geteuid()
	if defaultUID == 0 {
		defaultUID = syscall.Getuid()
		if defaultUID == 0 {
			defaultUID = 1000
		}
	}

	uid := defaultUID
	if !config.ReadNonnegInt(quiet.QUIET, indent, "PUID", &uid) {
		uid = defaultUID
	} else if uid == 0 {
		pp.Printf(indent, pp.EmojiUserError, "PUID cannot be 0. Using %d instead.", defaultUID)
		uid = defaultUID
	}

	// trying to raise cap.SETUID
	tryRaiseCap(cap.SETUID)

	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		pp.Printf(indent, pp.EmojiUserError, "Failed to set UID to %d: %v", uid, err)
	}
}

func dropCapabilities(indent pp.Indent) {
	if err := cap.NewSet().SetProc(); err != nil {
		pp.Printf(indent, pp.EmojiImpossible, "Failed to drop all capabilities: %v", err)
	}
}

// dropPriviledges drops all privileges as much as possible.
func dropPriviledges(indent pp.Indent) {
	pp.TopPrint(pp.EmojiPriviledges, "Dropping privileges . . .")

	// group ID
	dropSuperuserGroup(indent + 1)

	// user ID
	dropSuperuser(indent + 1)

	// all remaining capabilities
	dropCapabilities(indent + 1)
}

func printCapabilities(indent pp.Indent) {
	now, err := cap.GetPID(0)
	if err != nil {
		pp.Printf(indent, pp.EmojiImpossible, "Failed to get the current capabilities: %v", err)
	} else {
		diff, err := now.Compare(cap.NewSet())
		if err != nil {
			pp.Printf(indent, pp.EmojiImpossible, "Failed to compare capabilities: %v", err)
		} else if diff != 0 {
			pp.Printf(indent, pp.EmojiError, "The program still retains some additional capabilities: %v", now)
		}
	}
}

func printPriviledges(indent pp.Indent) {
	pp.TopPrint(pp.EmojiPriviledges, "Priviledges after dropping:")

	pp.Printf(indent+1, pp.EmojiBullet, "Effective UID:      %d", syscall.Geteuid())
	pp.Printf(indent+1, pp.EmojiBullet, "Effective GID:      %d", syscall.Getegid())

	switch groups, err := syscall.Getgroups(); {
	case err != nil:
		pp.Printf(indent+1, pp.EmojiImpossible, "Supplementary GIDs: (failed to get them)")
	case len(groups) > 0:
		pp.Printf(indent+1, pp.EmojiBullet, "Supplementary GIDs: %d\n", groups)
	default:
		pp.Print(indent+1, pp.EmojiBullet, "Supplementary GIDs: (empty)")
	}

	printCapabilities(indent + 1)
}
