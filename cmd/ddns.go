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
	"github.com/favonia/cloudflare-ddns-go/internal/detector"
)

func dropRoot() {
	log.Printf("🚷 Erasing supplementary group IDs . . .")
	err := syscall.Setgroups([]int{})
	if err != nil {
		log.Printf("😡 Could not erase supplementary group IDs: %v", err)
	}

	gid, err := common.GetenvAsInt("PGID", 1000)
	if err == nil {
		log.Printf("👪 Setting the group gid to %d . . .", gid)
		err := syscall.Setgid(gid)
		if err != nil {
			log.Printf("😡 Could not set the group ID: %v", err)
		}
	} else {
		log.Print(err)
	}

	uid, err := common.GetenvAsInt("PUID", 1000)
	if err == nil {
		log.Printf("🧑 Setting the user to %d . . .", uid)
		err := syscall.Setuid(uid)
		if err != nil {
			log.Printf("😡 Could not set the user ID: %v", err)
		}
	} else {
		log.Print(err)
	}

	log.Printf("🧑 Effective user ID of the process: %d.", os.Geteuid())
	log.Printf("👪 Effective group ID of the process: %d.", os.Getegid())

	if groups, err := syscall.Getgroups(); err == nil {
		log.Printf("👪 Supplementary group IDs of the process: %d.", groups)
	} else {
		log.Printf("😡 Could not get the supplementary group IDs.")
	}
	if os.Geteuid() == 0 || os.Getegid() == 0 {
		log.Printf("😰 It seems this program was run with root privilege. This is not recommended.")
	}
}

// returns true if the alarm is triggered before other signals come
func wait(signal chan os.Signal, d time.Duration) (continue_ bool) {
	chanAlarm := time.After(d)
	select {
	case sig := <-signal:
		log.Printf("👋 Caught signal: %v. Bye!", sig)
		return false
	case <-chanAlarm:
		return true
	}
}

func delayedExit(signal chan os.Signal) {
	duration := time.Minute
	log.Printf("🥱 Waiting for %v before exiting to prevent excessive logging . . .", duration)
	if continue_ := wait(signal, duration); continue_ {
		log.Printf("👋 Time's up. Bye!")
	}
	os.Exit(1)
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

mainLoop:
	for {
		ip4 := net.IP{}
		if c.IP4Policy != common.Unmanaged {
			ip, err := detector.GetIP4(c.IP4Policy)
			if err != nil {
				log.Print(err)
				log.Printf("🤔 Could not get the IPv4 address.")
			} else {
				log.Printf("🧐 Found the IPv4 address: %v", ip.To4())
				ip4 = ip
			}
		}

		ip6 := net.IP{}
		if c.IP6Policy != common.Unmanaged {
			ip, err := detector.GetIP6(c.IP6Policy)
			if err != nil {
				log.Print(err)
				log.Printf("🤔 Could not get the IPv6 address.")
			} else {
				log.Printf("🧐 Found the IPv6 address: %v", ip.To16())
				ip6 = ip
			}
		}

		for _, s := range c.Sites {
			h, err := s.Handler.Handle()
			if err != nil {
				log.Print(err)
				continue
			}
			for _, target := range s.Targets {
				err := h.Update(&api.UpdateArgs{
					Context:    ctx,
					Target:     target,
					IP4Managed: c.IP4Policy != common.Unmanaged,
					IP4:        ip4,
					IP6Managed: c.IP6Policy != common.Unmanaged,
					IP6:        ip6,
					TTL:        s.TTL,
					Proxied:    s.Proxied,
				})
				if err != nil {
					log.Print(err)
				}
			}
		}

		if !c.Quiet {
			log.Printf("😴 Checking the IPs again in %s . . .", c.RefreshInterval.String())
		}

		if continue_ := wait(chanSignal, c.RefreshInterval); continue_ {
			continue mainLoop
		} else {
			break mainLoop
		}
	}
}
