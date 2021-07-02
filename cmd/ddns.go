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

func main() {
	ctx := context.Background()

	// catching SIGINT and SIGTERM
	chanSignal := make(chan os.Signal, 1)
	signal.Notify(chanSignal, syscall.SIGINT, syscall.SIGTERM)

	// reading the config
	c, err := ddns.ReadEnv()
	if err != nil {
		log.Print(err)
		log.Printf("🕒 Waiting for one minute before exiting to prevent excessive logging.")
		if continue_ := wait(chanSignal, time.Minute); continue_ {
			log.Printf("🕒 Time's up. Bye!")
		}
		os.Exit(1)
	}

	// preparing the CloadFlare client
	h, err := ddns.NewAPI(c.Token)
	if err != nil {
		log.Print(err)
		log.Printf("🕒 Waiting for one minute before exiting to prevent excessive logging.")
		if continue_ := wait(chanSignal, time.Minute); continue_ {
			log.Printf("🕒 Time's up. Bye!")
		}
		os.Exit(1)
	}

mainLoop:
	for {
		var noIP net.IP = nil
		var ip4 *net.IP = nil
		if c.IP4Policy != ddns.Disabled {
			ip, err := ddns.GetIP4(c.IP4Policy)
			if err != nil {
				log.Print(err)
				log.Printf("❓ Could not get IP4 address")
				ip4 = &noIP
			} else {
				log.Printf("🔍 Found the IP4 address: %v", ip.To4())
				ip4 = &ip
			}
		}

		var ip6 *net.IP = nil
		if c.IP6Policy != ddns.Disabled {
			ip, err := ddns.GetIP6(c.IP6Policy)
			if err != nil {
				log.Print(err)
				log.Printf("❓Could not get IP6 address")
				ip6 = &noIP
			} else {
				log.Printf("🔍 Found the IP6 address: %v", ip.To16())
				ip6 = &ip
			}
		}

		for _, fqdn := range c.Domains {
			s := ddns.DNSSetting{
				FQDN:    fqdn,
				IP4:     ip4,
				IP6:     ip6,
				TTL:     c.TTL,
				Proxied: c.Proxied,
			}
			err := h.UpdateDNSRecords(ctx, s)
			if err != nil {
				log.Print(err)
			}
		}

		log.Printf("🕰️ Checking the DNS records again in %s . . .", c.RefreshInterval.String())

		if continue_ := wait(chanSignal, c.RefreshInterval); continue_ {
			continue mainLoop
		} else {
			break mainLoop
		}
	}
}
