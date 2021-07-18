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
	log.Printf("🥱 Waiting for %v before exiting to prevent excessive looping.", duration)
	log.Printf("🥱 Press Ctrl+C to exit immediately . . .")
	if sig := wait(signal, duration); sig == nil {
		log.Printf("👋 Time's up. Bye!")
	} else {
		log.Printf("👋 Caught signal: %v. Bye!", *sig)
	}
	os.Exit(1)
}

func main() {
	// dropping the superuser privilege
	dropPriviledges()
	printPriviledges()

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
				log.Printf("😴 Checking the IP addresses again in %v . . .", cron.PPDuration(interval))
			} else {
				log.Printf("😴 Checking the IP addresses in %v . . .", cron.PPDuration(interval))
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
