package main

import (
	"fmt"
	"syscall"

	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/favonia/cloudflare-ddns-go/internal/config"
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

func dropSuperuserGroup() {
	defaultGID := syscall.Getegid()
	if defaultGID == 0 {
		defaultGID = syscall.Getgid() // real group ID
		if defaultGID == 0 {
			defaultGID = 1000
		}
	}

	gid := defaultGID
	if !config.ReadNonnegInt(quiet.QUIET, "PGID", &gid) {
		gid = defaultGID
	} else if gid == 0 {
		fmt.Printf("😡 PGID cannot be 0. Using %d instead.\n", defaultGID)
		gid = defaultGID
	}

	// trying to raise cap.SETGID
	tryRaiseCap(cap.SETGID)

	if err := syscall.Setgroups([]int{}); err != nil {
		fmt.Printf("🤔 Could not erase all supplementary gruop IDs: %v\n", err)
	}

	if err := syscall.Setresgid(gid, gid, gid); err != nil {
		fmt.Printf("🤔 Could not set the group ID to %d: %v\n", gid, err)
	}
}

func dropSuperuser() {
	defaultUID := syscall.Geteuid()
	if defaultUID == 0 {
		defaultUID = syscall.Getuid()
		if defaultUID == 0 {
			defaultUID = 1000
		}
	}

	uid := defaultUID
	if !config.ReadNonnegInt(quiet.QUIET, "PUID", &uid) {
		uid = defaultUID
	} else if uid == 0 {
		fmt.Printf("😡 PUID cannot be 0. Using %d instead.\n", defaultUID)
		uid = defaultUID
	}

	// trying to raise cap.SETUID
	tryRaiseCap(cap.SETUID)

	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		fmt.Printf("🤔 Could not set the user ID to %d: %v\n", uid, err)
	}
}

func dropCapabilities() {
	if err := cap.NewSet().SetProc(); err != nil {
		fmt.Printf("😡 Could not drop all capabilities: %v\n", err)
	}
}

// dropPriviledges drops all privileges as much as possible.
func dropPriviledges() {
	// group ID
	dropSuperuserGroup()

	// user ID
	dropSuperuser()

	// all remaining capabilities
	dropCapabilities()
}

func printCapabilities() {
	now, err := cap.GetPID(0)
	if err != nil {
		fmt.Printf("🤯 Could not get the current capabilities: %v\n", err)
	} else {
		diff, err := now.Compare(cap.NewSet())
		if err != nil {
			fmt.Printf("🤯 Could not compare capabilities: %v\n", err)
		} else if diff != 0 {
			fmt.Printf("😰 The program still retains some additional capabilities: %v\n", now)
		}
	}
}

func printPriviledges() {
	fmt.Printf("🧑 Effective user ID: %d.\n", syscall.Geteuid())
	fmt.Printf("👪 Effective group ID: %d.\n", syscall.Getegid())

	switch groups, err := syscall.Getgroups(); {
	case err != nil:
		fmt.Printf("😡 Could not get the supplementary group IDs.\n")
	case len(groups) > 0:
		fmt.Printf("👪 Supplementary group IDs: %d.\n", groups)
	default:
		fmt.Printf("👪 No supplementary group IDs.\n")
	}

	if syscall.Geteuid() == 0 || syscall.Getegid() == 0 {
		fmt.Printf("😰 The program is still run as the superuser.\n")
	}

	printCapabilities()
}
