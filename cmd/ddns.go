package main

import (
	"context"
	"fmt"
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
		fmt.Printf("🌟 CloudFlare DDNS\n")
		return
	}

	fmt.Printf("🌟 CloudFlare DDNS (%s)\n", Version)
}

func initConfig(ctx context.Context) (*config.Config, *api.Handle) {
	// reading the config
	c := config.Default()
	if !c.ReadEnv() {
		exit()
	}
	if !c.Normalize() {
		exit()
	}

	if !c.Quiet {
		config.PrintConfig(c)
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
				fmt.Printf("🚨 No scheduled updates in near future. Deleting all managed records . . .\n")
				clearIPs(ctx, c, h)
				fmt.Printf("👋 Done now. Bye!\n")
			} else {
				fmt.Printf("🚨 No scheduled updates in near future. Bye!\n")
			}

			break mainLoop
		}

		interval := time.Until(next)
		if !c.Quiet {
			switch {
			case interval < -IntervalLargeGap:
				fmt.Printf("🏃 Checking the IP addresses now (running behind by %v) . . .\n", -interval.Round(IntervalUnit))
			case interval < IntervalUnit:
				fmt.Printf("🏃 Checking the IP addresses now . . .\n")
			case interval < IntervalLargeGap:
				fmt.Printf("🏃 Checking the IP addresses in less than %v . . .\n", IntervalLargeGap)
			default:
				fmt.Printf("💤 Checking the IP addresses in about %v . . .\n", interval.Round(IntervalUnit))
			}
		}

		if sig, ok := signalWait(chanSignal, interval); !ok {
			continue mainLoop
		} else {
			switch sig.(syscall.Signal) {
			case syscall.SIGHUP:
				fmt.Printf("🚨 Caught signal: %v.\n", sig)
				h.FlushCache()

				fmt.Printf("🔁 Restarting . . .\n")
				c, h = initConfig(ctx)
				continue mainLoop

			case syscall.SIGINT, syscall.SIGTERM:
				if c.DeleteOnStop {
					fmt.Printf("🚨 Caught signal: %v. Deleting all managed records . . .\n", sig)
					clearIPs(ctx, c, h)
					fmt.Printf("👋 Done now. Bye!\n")
				} else {
					fmt.Printf("🚨 Caught signal: %v. Bye!\n", sig)
				}

				break mainLoop

			default:
				fmt.Printf("🚨 Caught unexpected signal: %v.\n", sig)
				continue mainLoop
			}
		}
	}
}
