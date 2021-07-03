package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal"
)

func dropRoot() {
	log.Printf("🚷 Erasing supplementary group IDs.")
	syscall.Setgroups([]int{})

	gid, err := ddns.GetenvAsInt("PGID", 1000)
	if err == nil {
		log.Printf("👪 Setting the gid to %d", gid)
		syscall.Setgid(gid)
	} else {
		log.Print(err)
	}

	uid, err := ddns.GetenvAsInt("PUID", 1000)
	if err == nil {
		log.Printf("🧑 Setting the uid to %d", uid)
		syscall.Setuid(uid)
	} else {
		log.Print(err)
	}

	log.Printf("🧑 Effective user ID of the process: %d", os.Geteuid())
	log.Printf("👪 Effective group ID of the process: %d", os.Getegid())

	if groups, err := syscall.Getgroups(); err == nil {
		log.Printf("👪 Supplementary group IDs of the process: %d", groups)
	} else {
		log.Printf("😡 Could not get supplementary group IDs.")
	}
	if os.Geteuid() == 0 || os.Getegid() == 0 {
		log.Printf("⚠️ It seems this program was run with root privilege. This is not recommended.")
	}
}

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
	log.Printf("⏰ Waiting for one minute before exiting to prevent excessive logging.")
	if continue_ := wait(signal, time.Minute); continue_ {
		log.Printf("⏰ Time's up. Bye!")
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
	c, err := ddns.ReadEnv()
	if err != nil {
		log.Print(err)
		delayedExit(chanSignal)
	}

	// preparing the CloadFlare client
	h, err := ddns.NewAPI(c.Token)
	if err != nil {
		log.Print(err)
		delayedExit(chanSignal)
	}

mainLoop:
	for {
		ip4 := net.IP(nil)
		if c.IP4Policy != ddns.Unmanaged {
			ip, err := ddns.GetIP4(c.IP4Policy)
			if err != nil {
				log.Print(err)
				log.Printf("🤔 Could not get IP4 address")
			} else {
				log.Printf("🔍 Found the IP4 address: %v", ip.To4())
				ip4 = ip
			}
		}

		ip6 := net.IP(nil)
		if c.IP6Policy != ddns.Unmanaged {
			ip, err := ddns.GetIP6(c.IP6Policy)
			if err != nil {
				log.Print(err)
				log.Printf("🤔 Could not get IP6 address")
			} else {
				log.Printf("🔍 Found the IP6 address: %v", ip.To16())
				ip6 = ip
			}
		}

		for _, fqdn := range c.Domains {
			s := ddns.DNSSetting{
				FQDN:       fqdn,
				IP4Managed: c.IP4Policy != ddns.Unmanaged,
				IP4:        ip4,
				IP6Managed: c.IP6Policy != ddns.Unmanaged,
				IP6:        ip6,
				TTL:        c.TTL,
				Proxied:    c.Proxied,
			}
			err := h.UpdateDNSRecords(ctx, &s)
			if err != nil {
				log.Print(err)
			}
		}

		log.Printf("⏰ Checking the DNS records again in %s . . .", c.RefreshInterval.String())

		if continue_ := wait(chanSignal, c.RefreshInterval); continue_ {
			continue mainLoop
		} else {
			break mainLoop
		}
	}
}
