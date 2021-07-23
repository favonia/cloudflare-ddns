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
)

const (
	IntervalUnit     = time.Second
	IntervalLargeGap = time.Second * 10
)

// signalWait returns false if the alarm is triggered before other signals come.
func signalWait(signal chan os.Signal, d time.Duration) (os.Signal, bool) {
	chanAlarm := time.After(d)
	select {
	case sig := <-signal:
		return sig, true
	case <-chanAlarm:
		return nil, false
	}
}

func exit() {
	os.Exit(1)
}

var Version string //nolint:gochecknoglobals

func welcome() {
	if Version == "" {
		log.Printf("🌟 CloudFlare DDNS")
		return
	}

	log.Printf("🌟 CloudFlare DDNS (%s)", Version)
}

func initConfig(ctx context.Context) (*config.Config, *api.Handle) {
	// reading the config
	c, ok := config.ReadConfig(ctx)
	if !ok {
		exit()
	}

	if !c.Quiet {
		config.PrintConfig(ctx, c)
	}

	// getting the handler
	h, ok := c.Auth.New(ctx, c.CacheExpiration)
	if !ok {
		exit()
	}

	return c, h
}

func main() { //nolint:funlen,gocognit,cyclop
	welcome()

	// dropping the superuser privilege
	dropPriviledges()
	printPriviledges()

	ctx := context.Background()

	// catching SIGINT and SIGTERM
	chanSignal := make(chan os.Signal, 1)
	signal.Notify(chanSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// reading the config
	c, h := initConfig(ctx)

	first := true
mainLoop:
	for {
		next := c.UpdateCron.Next()
		if !first || c.UpdateOnStart {
			updateIPs(ctx, c, h)
		}
		first = false

		if next.IsZero() {
			if c.DeleteOnStop {
				log.Printf("🚨 No scheduled updates in near future. Deleting all managed records . . .")
				clearIPs(ctx, c, h)
				log.Printf("👋 Done now. Bye!")
			} else {
				log.Printf("🚨 No scheduled updates in near future. Bye!")
			}

			break mainLoop
		}

		interval := time.Until(next)
		if !c.Quiet {
			switch {
			case interval < -IntervalLargeGap:
				log.Printf("🏃 Checking the IP addresses now (running behind by %v) . . .", -interval.Round(IntervalUnit))
			case interval < IntervalUnit:
				log.Printf("🏃 Checking the IP addresses now . . .")
			case interval < IntervalLargeGap:
				log.Printf("🏃 Checking the IP addresses in less than %v . . .", IntervalLargeGap)
			default:
				log.Printf("💤 Checking the IP addresses in about %v . . .", interval.Round(IntervalUnit))
			}
		}

		if sig, ok := signalWait(chanSignal, interval); !ok {
			continue mainLoop
		} else {
			switch sig.(syscall.Signal) {
			case syscall.SIGHUP:
				log.Printf("🚨 Caught signal: %v.", sig)
				h.FlushCache()

				log.Printf("🔁 Restarting . . .")
				c, h = initConfig(ctx)
				continue mainLoop

			case syscall.SIGINT, syscall.SIGTERM:
				if c.DeleteOnStop {
					log.Printf("🚨 Caught signal: %v. Deleting all managed records . . .", sig)
					clearIPs(ctx, c, h)
					log.Printf("👋 Done now. Bye!")
				} else {
					log.Printf("🚨 Caught signal: %v. Bye!", sig)
				}

				break mainLoop

			default:
				log.Printf("🚨 Caught unexpected signal: %v.", sig)
				continue mainLoop
			}
		}
	}
}
