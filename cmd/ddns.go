package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
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

func dropRoot() {
	// group ID
	{
		defaultGID := syscall.Getegid()
		if defaultGID == 0 {
			defaultGID = syscall.Getgid() // real group ID
			if defaultGID == 0 {
				defaultGID = 1000
			}
		}
		gid, err := config.GetenvAsInt("PGID", defaultGID, quiet.QUIET)
		if err != nil {
			log.Print(err)
			gid = defaultGID
		} else if gid == 0 {
			log.Printf("😡 PGID cannot be 0. Using %d instead . . .", defaultGID)
			gid = defaultGID
		}

		// trying to raise cap.SETGID
		tryRaiseCap(cap.SETGID)
		if err = syscall.Setgroups([]int{}); err != nil {
			log.Printf("🤔 Could not erase all supplementary gruop IDs: %v", err)
		}
		if err = syscall.Setresgid(gid, gid, gid); err != nil {
			log.Printf("🤔 Could not set the group ID to %d: %v", gid, err)
		}
	}

	// user ID
	{
		defaultUID := syscall.Geteuid()
		if defaultUID == 0 {
			defaultUID = syscall.Getuid()
			if defaultUID == 0 {
				defaultUID = 1000
			}
		}
		uid, err := config.GetenvAsInt("PUID", defaultUID, quiet.QUIET)
		if err != nil {
			log.Print(err)
			uid = defaultUID
		} else if uid == 0 {
			log.Printf("😡 PUID cannot be 0. Using %d instead . . .", defaultUID)
			uid = defaultUID
		}

		// trying to raise cap.SETUID
		tryRaiseCap(cap.SETUID)
		if err = syscall.Setresuid(uid, uid, uid); err != nil {
			log.Printf("🤔 Could not set the user ID to %d: %v", uid, err)
		}
	}

	if err := cap.NewSet().SetProc(); err != nil {
		log.Printf("😡 Could not drop all privileges: %v", err)
	}

	log.Printf("🧑 Effective user ID: %d.", syscall.Geteuid())
	log.Printf("👪 Effective group ID: %d.", syscall.Getegid())

	if groups, err := syscall.Getgroups(); err != nil {
		log.Printf("😡 Could not get the supplementary group IDs.")
	} else if len(groups) > 0 {
		log.Printf("😰 The program still has supplementary group IDs: %d.", groups)
	}
	if syscall.Geteuid() == 0 || syscall.Getegid() == 0 {
		log.Printf("😰 The program is still run as the root.")
	}

	{
		now, err := cap.GetPID(0)
		if err != nil {
			log.Printf("🤯 Could not get the current capacities: %v", err)
		} else {
			diff, err := now.Compare(cap.NewSet())
			if err != nil {
				log.Printf("🤯 Could not compare capacities: %v", err)
			} else if diff != 0 {
				log.Printf("😰 The program still retains some additional capacities: %v", now)
			}
		}
	}
}

// returns true if the alarm is triggered before other signals come
func wait(signal chan os.Signal, d time.Duration) *os.Signal {
	chanAlarm := time.After(d)
	select {
	case sig := <-signal:
		return &sig
	case <-chanAlarm:
		return nil
	}
}

func delayedExit(signal chan os.Signal) {
	duration := time.Minute * 2
	log.Printf("🥱 Waiting for %v before exiting to prevent excessive looping when used with Docker Compose.", duration)
	log.Printf("🥱 Press Ctrl+C to exit immediately . . .")
	if sig := wait(signal, duration); sig == nil {
		log.Printf("👋 Time's up. Bye!")
	} else {
		log.Printf("👋 Caught signal: %v. Bye!", *sig)
	}
	os.Exit(1)
}

func setIPs(ctx context.Context, c *config.Config, h *api.Handle, ip4 net.IP, ip6 net.IP) {
	for _, target := range c.Targets {
		err := h.Update(&api.UpdateArgs{
			Context:    ctx,
			Target:     target,
			IP4Managed: c.IP4Policy.IsManaged(),
			IP4:        ip4,
			IP6Managed: c.IP6Policy.IsManaged(),
			IP6:        ip6,
			TTL:        c.TTL,
			Proxied:    c.Proxied,
			Quiet:      c.Quiet,
		})
		if err != nil {
			log.Print(err)
		}
	}
	for _, target := range c.IP4Targets {
		err := h.Update(&api.UpdateArgs{
			Context:    ctx,
			Target:     target,
			IP4Managed: c.IP4Policy.IsManaged(),
			IP4:        ip4,
			IP6Managed: false,
			IP6:        nil,
			TTL:        c.TTL,
			Proxied:    c.Proxied,
			Quiet:      c.Quiet,
		})
		if err != nil {
			log.Print(err)
		}
	}
	for _, target := range c.IP6Targets {
		err := h.Update(&api.UpdateArgs{
			Context:    ctx,
			Target:     target,
			IP4Managed: false,
			IP4:        nil,
			IP6Managed: c.IP6Policy.IsManaged(),
			IP6:        ip6,
			TTL:        c.TTL,
			Proxied:    c.Proxied,
			Quiet:      c.Quiet,
		})
		if err != nil {
			log.Print(err)
		}
	}
}

func updateIPs(ctx context.Context, c *config.Config, h *api.Handle) {
	var ip4 net.IP
	if c.IP4Policy.IsManaged() {
		ip, err := c.IP4Policy.GetIP4(c.DetectionTimeout)
		if err != nil {
			log.Print(err)
			log.Printf("🤔 Could not get the IPv4 address.")
		} else {
			if !c.Quiet {
				log.Printf("🧐 Found the IPv4 address: %v", ip.To4())
			}
			ip4 = ip
		}
	}

	var ip6 net.IP
	if c.IP6Policy.IsManaged() {
		ip, err := c.IP6Policy.GetIP6(c.DetectionTimeout)
		if err != nil {
			log.Print(err)
			log.Printf("🤔 Could not get the IPv6 address.")
		} else {
			if !c.Quiet {
				log.Printf("🧐 Found the IPv6 address: %v", ip.To16())
			}
			ip6 = ip
		}
	}

	setIPs(ctx, c, h, ip4, ip6)
}

func clearIPs(ctx context.Context, c *config.Config, h *api.Handle) {
	setIPs(ctx, c, h, nil, nil)
}

func main() {
	// dropping the root privilege
	dropRoot()

	ctx := context.Background()

	// catching SIGINT and SIGTERM
	chanSignal := make(chan os.Signal, 1)
	signal.Notify(chanSignal, syscall.SIGINT, syscall.SIGTERM)

	// reading the config
	c, err := config.ReadConfig(ctx)
	if err != nil {
		log.Print(err)
		delayedExit(chanSignal)
	}

	// (re)initiating the cache
	api.InitCache(c.CacheExpiration)

	// getting the handler
	h, err := c.Auth.New()
	if err != nil {
		log.Print(err)
		delayedExit(chanSignal)
	}

mainLoop:
	for first := true; ; first = false {
		next := c.RefreshCron.Next()
		if !first || c.RefreshOnStart {
			updateIPs(ctx, c, h)
		}

		if next.IsZero() {
			if c.DeleteOnStop {
				log.Printf("😮 No future updating scheduled. Deleting all managed records . . .")
				clearIPs(ctx, c, h)
				log.Printf("👋 Done now. Bye!")
			} else {
				log.Printf("👋 No future updating scheduled. Bye!")
			}
			break mainLoop
		}

		interval := time.Until(next)
		if interval <= 0 {
			if !c.Quiet {
				log.Printf("😪 Running behind the schedule by %s . . .", -interval)
			}
			interval = 0
		}

		if !c.Quiet {
			if first {
				log.Printf("😴 Checking the IP addresses in %v . . .", interval)
			} else {
				log.Printf("😴 Checking the IP addresses again in %v . . .", interval)
			}
		}
		if sig := wait(chanSignal, interval); sig == nil {
			continue mainLoop
		} else {
			if c.DeleteOnStop {
				log.Printf("😮 Caught signal: %v. Deleting all managed records . . .", *sig)
				clearIPs(ctx, c, h) // `nil` to purge all records
				log.Printf("👋 Done now. Bye!")
			} else {
				log.Printf("👋 Caught signal: %v. Bye!", *sig)
			}
			break mainLoop
		}
	}
}
