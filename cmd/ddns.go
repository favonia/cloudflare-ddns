package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/config"
	"github.com/favonia/cloudflare-ddns-go/internal/cron"
)

// wait returns true if the alarm is triggered before other signals come.
func wait(signal chan os.Signal, d time.Duration) *os.Signal {
	chanAlarm := time.After(d)
	select {
	case sig := <-signal:
		return &sig
	case <-chanAlarm:
		return nil
	}
}

func exit() {
	os.Exit(1)
}

var Version string = ""

func welcome() {
	if Version == "" {
		log.Printf("🌟 CloudFlare DDNS")
		return
	}

	log.Printf("🌟 CloudFlare DDNS version %s", Version)
}

func main() {
	welcome()

	// dropping the superuser privilege
	dropPriviledges()
	printPriviledges()

	ctx := context.Background()

	// catching SIGINT and SIGTERM
	chanSignal := make(chan os.Signal, 1)
	signal.Notify(chanSignal, syscall.SIGINT, syscall.SIGTERM)

	// reading the config
	c, ok := config.ReadConfig(ctx)
	if !ok {
		exit()
	}

	// (re)initiating the cache
	api.InitCache(c.CacheExpiration)

	// getting the handler
	h, ok := c.Auth.New()
	if !ok {
		exit()
	}

	first := true
	updated := false
mainLoop:
	for {
		next := c.RefreshCron.Next()
		if !first || c.RefreshOnStart {
			updateIPs(ctx, c, h)
			updated = true
		}
		first = false

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
			if updated {
				log.Printf("😴 Checking the IP addresses again %v . . .", cron.PrintPhrase(interval))
			} else {
				log.Printf("😴 Checking the IP addresses %v . . .", cron.PrintPhrase(interval))
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
