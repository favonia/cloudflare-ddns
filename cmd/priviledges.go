package main

import (
	"log"
	"syscall"

	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/favonia/cloudflare-ddns-go/internal/config"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

func tryRaiseCap(val cap.Value) {
	c, err := cap.GetPID(0)
	if err != nil {
		return
	}
	if err := c.SetFlag(cap.Effective, true, cap.SETGID); err != nil {
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
	gid, ok := config.GetenvAsInt("PGID", defaultGID, quiet.QUIET)
	if !ok {
		gid = defaultGID
	} else if gid == 0 {
		log.Printf("😡 PGID cannot be 0. Using %d instead . . .", defaultGID)
		gid = defaultGID
	}

	// trying to raise cap.SETGID
	tryRaiseCap(cap.SETGID)
	if err := syscall.Setgroups([]int{}); err != nil {
		log.Printf("🤔 Could not erase all supplementary gruop IDs: %v", err)
	}
	if err := syscall.Setresgid(gid, gid, gid); err != nil {
		log.Printf("🤔 Could not set the group ID to %d: %v", gid, err)
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
	uid, ok := config.GetenvAsInt("PUID", defaultUID, quiet.QUIET)
	if !ok {
		uid = defaultUID
	} else if uid == 0 {
		log.Printf("😡 PUID cannot be 0. Using %d instead . . .", defaultUID)
		uid = defaultUID
	}

	// trying to raise cap.SETUID
	tryRaiseCap(cap.SETUID)
	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		log.Printf("🤔 Could not set the user ID to %d: %v", uid, err)
	}
}

func dropCapabilities() {
	if err := cap.NewSet().SetProc(); err != nil {
		log.Printf("😡 Could not drop all capabilities: %v", err)
	}
}

// dropPriviledges drops all privileges as much as possible
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
		log.Printf("🤯 Could not get the current capabilities: %v", err)
	} else {
		diff, err := now.Compare(cap.NewSet())
		if err != nil {
			log.Printf("🤯 Could not compare capabilities: %v", err)
		} else if diff != 0 {
			log.Printf("😰 The program still retains some additional capabilities: %v", now)
		}
	}
}

func printPriviledges() {
	log.Printf("🧑 Effective user ID: %d.", syscall.Geteuid())
	log.Printf("👪 Effective group ID: %d.", syscall.Getegid())

	switch groups, err := syscall.Getgroups(); {
	case err != nil:
		log.Printf("😡 Could not get the supplementary group IDs.")
	case len(groups) > 0:
		log.Printf("👪 Supplementary group IDs: %d.", groups)
	default:
		log.Printf("👪 No supplementary group IDs.")
	}

	if syscall.Geteuid() == 0 || syscall.Getegid() == 0 {
		log.Printf("😰 The program is still run as the superuser.")
	}

	printCapabilities()
}
