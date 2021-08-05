package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/pp"
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

var Version string //nolint:gochecknoglobals

func welcome() {
	if Version == "" {
		pp.TopPrintf(pp.EmojiStar, "Cloudflare DDNS")
		return
	}

	pp.TopPrintf(pp.EmojiStar, "Cloudflare DDNS (%s)", Version)
}

func initConfig(ctx context.Context) (*config.Config, api.Handle) {
	// reading the config
	c := config.Default()
	if !c.ReadEnv(pp.NoIndent) {
		pp.TopPrintf(pp.EmojiBye, "Bye!")
		os.Exit(1)
	}
	if !c.Normalize(pp.NoIndent) {
		pp.TopPrintf(pp.EmojiBye, "Bye!")
		os.Exit(1)
	}

	if !c.Quiet {
		config.PrintConfig(pp.NoIndent, c)
	}

	// getting the handler
	h, ok := c.Auth.New(ctx, pp.NoIndent, c.CacheExpiration)
	if !ok {
		pp.TopPrintf(pp.EmojiBye, "Bye!")
		os.Exit(1)
	}

	return c, h
}

func main() { //nolint:funlen,gocognit,cyclop
	welcome()

	// dropping the superuser privilege
	dropPriviledges(pp.NoIndent)

	// printing the current privileges
	printPriviledges(pp.NoIndent)

	// catching SIGINT and SIGTERM
	chanSignal := make(chan os.Signal, 1)
	signal.Notify(chanSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// context
	ctx := context.Background()

	// reading the config
	c, h := initConfig(ctx)

	first := true
mainLoop:
	for {
		next := c.UpdateCron.Next()
		if !first || c.UpdateOnStart {
			updateIPs(ctx, pp.NoIndent, c, h)
		}
		first = false

		if next.IsZero() {
			if c.DeleteOnStop {
				pp.TopPrintf(pp.EmojiUserError, "No scheduled updates in near future. Deleting all managed records . . .")
				clearIPs(ctx, pp.NoIndent, c, h)
				pp.TopPrintf(pp.EmojiBye, "Done now. Bye!")
			} else {
				pp.TopPrintf(pp.EmojiUserError, "No scheduled updates in near future.")
				pp.TopPrintf(pp.EmojiBye, "Bye!")
			}

			break mainLoop
		}

		interval := time.Until(next)
		if !c.Quiet {
			switch {
			case interval < -IntervalLargeGap:
				pp.TopPrintf(pp.EmojiNow, "Checking the IP addresses now (running behind by %v) . . .",
					-interval.Round(IntervalUnit))
			case interval < IntervalUnit:
				pp.TopPrintf(pp.EmojiNow, "Checking the IP addresses now . . .")
			case interval < IntervalLargeGap:
				pp.TopPrintf(pp.EmojiNow, "Checking the IP addresses in less than %v . . .", IntervalLargeGap)
			default:
				pp.TopPrintf(pp.EmojiAlarm, "Checking the IP addresses in about %v . . .", interval.Round(IntervalUnit))
			}
		}

		if sig, ok := signalWait(chanSignal, interval); !ok {
			continue mainLoop
		} else {
			switch sig.(syscall.Signal) {
			case syscall.SIGHUP:
				pp.TopPrintf(pp.EmojiSignal, "Caught signal: %v.", sig)
				h.FlushCache()

				pp.TopPrintf(pp.EmojiNow, "Restarting . . .")
				c, h = initConfig(ctx)
				continue mainLoop

			case syscall.SIGINT, syscall.SIGTERM:
				if c.DeleteOnStop {
					pp.TopPrintf(pp.EmojiSignal, "Caught signal: %v. Deleting all managed records . . .", sig)
					clearIPs(ctx, pp.NoIndent, c, h)
					pp.TopPrintf(pp.EmojiBye, "Done now. Bye!")
				} else {
					pp.TopPrintf(pp.EmojiSignal, "Caught signal: %v.", sig)
					pp.TopPrintf(pp.EmojiBye, "Bye!")
				}

				break mainLoop

			default:
				pp.TopPrintf(pp.EmojiSignal, "Caught and ignored unexpected signal: %v.", sig)
				continue mainLoop
			}
		}
	}
}
