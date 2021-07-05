package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/common"
	"github.com/favonia/cloudflare-ddns-go/internal/config"
)

func dropRoot() {
	{
		log.Printf("🚷 Erasing supplementary group IDs . . .")
		err := syscall.Setgroups([]int{})
		if err != nil {
			log.Printf("🤷 Could not erase supplementary group IDs: %v", err)
		}
	}

	// group ID
	{
		var (
			currentGid = syscall.Getegid()
			defaultGid = currentGid
		)
		if defaultGid == 0 {
			defaultGid = syscall.Getgid() // real group ID
			if defaultGid == 0 {
				defaultGid = 1000
			}
		}
		gid, err := config.GetenvAsInt("PGID", defaultGid, common.VERBOSE)
		if err == nil {
			if gid != currentGid {
				log.Printf("👪 Setting the group ID to %d . . .", gid)
				err := syscall.Setgid(gid)
				if err != nil {
					log.Printf("😡 Could not set the group ID: %v", err)
				}
			}
		} else {
			log.Print(err)
		}
	}

	// user ID
	{
		var (
			currentUid = syscall.Geteuid()
			defaultUid = currentUid
		)
		if defaultUid == 0 {
			defaultUid = syscall.Getuid()
			if defaultUid == 0 {
				defaultUid = 1000
			}
		}
		uid, err := config.GetenvAsInt("PUID", defaultUid, common.VERBOSE)
		if err == nil {
			if uid != currentUid {
				log.Printf("🧑 Setting the user ID to %d . . .", uid)
				err := syscall.Setuid(uid)
				if err != nil {
					log.Printf("😡 Could not set the user ID: %v", err)
				}
			}
		} else {
			log.Print(err)
		}
	}

	log.Printf("🧑 Effective user ID of the process: %d.", syscall.Geteuid())
	log.Printf("👪 Effective group ID of the process: %d.", syscall.Getegid())

	if groups, err := syscall.Getgroups(); err == nil {
		log.Printf("👪 Supplementary group IDs of the process: %d.", groups)
	} else {
		log.Printf("😡 Could not get the supplementary group IDs.")
	}
	if syscall.Geteuid() == 0 || syscall.Getegid() == 0 {
		log.Printf("😰 It seems this program was run with root privilege. This is not recommended.")
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
	log.Printf("🥱 Waiting for %v before exiting to prevent excessive looping . . .", duration)
	if sig := wait(signal, duration); sig == nil {
		log.Printf("👋 Time's up. Bye!")
	} else {
		log.Printf("👋 Caught signal: %v. Bye!", *sig)
	}
	os.Exit(1)
}

func applyIPs(ctx context.Context, c *config.Config, h *api.Handle, ip4 net.IP, ip6 net.IP) {
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
	h, err := c.NewHandler.NewHandle()
	if err != nil {
		log.Print(err)
		delayedExit(chanSignal)
	}

mainLoop:
	for {
		ip4 := net.IP{}
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

		ip6 := net.IP{}
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

		applyIPs(ctx, c, h, ip4, ip6)

		if !c.Quiet {
			log.Printf("😴 Checking the IP addresses again in %v . . .", c.RefreshInterval)
		}
		if sig := wait(chanSignal, c.RefreshInterval); sig == nil {
			continue mainLoop
		} else {
			if c.DeleteOnExit {
				log.Printf("😮 Caught signal: %v. Deleting all managed records . . .", *sig)
				applyIPs(ctx, c, h, nil, nil) // `nil` to purge all records
				log.Printf("👋 Done now. Bye!")
			} else {
				log.Printf("👋 Caught signal: %v. Bye!", *sig)
			}
			break mainLoop
		}
	}
}
