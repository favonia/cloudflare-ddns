package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
	"github.com/favonia/cloudflare-ddns/internal/updater"
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

func welcome(ppfmt pp.PP) {
	if Version == "" {
		ppfmt.Noticef(pp.EmojiStar, "Cloudflare DDNS")
		return
	}

	ppfmt.Noticef(pp.EmojiStar, "Cloudflare DDNS (%s)", Version)
}

func initConfig(ctx context.Context, ppfmt pp.PP) (*config.Config, api.Handle, setter.Setter) {
	// reading the config
	c := config.Default()
	if !c.ReadEnv(ppfmt) {
		ppfmt.Noticef(pp.EmojiBye, "Bye!")
		os.Exit(1)
	}
	if !c.NormalizeDomains(ppfmt) {
		ppfmt.Noticef(pp.EmojiBye, "Bye!")
		monitor.ExitStatusAll(ctx, ppfmt, c.Monitors, 1)
		os.Exit(1)
	}

	c.Print(ppfmt)

	// getting the handler
	h, ok := c.Auth.New(ctx, ppfmt, c.CacheExpiration)
	if !ok {
		ppfmt.Noticef(pp.EmojiBye, "Bye!")
		monitor.ExitStatusAll(ctx, ppfmt, c.Monitors, 1)
		os.Exit(1)
	}

	// getting the setter
	s, ok := setter.New(ppfmt, h, c.TTL, c.Proxied)
	if !ok {
		ppfmt.Noticef(pp.EmojiBye, "Bye!")
		monitor.ExitStatusAll(ctx, ppfmt, c.Monitors, 1)
		os.Exit(1)
	}

	return c, h, s
}

func main() { //nolint:funlen,cyclop,gocognit
	ppfmt := pp.New(os.Stdout)
	if !config.ReadQuiet("QUIET", &ppfmt) {
		ppfmt.Noticef(pp.EmojiUserError, "Bye!")
		return
	}
	if !ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Noticef(pp.EmojiMute, "Quiet mode enabled")
	}

	welcome(ppfmt)

	// dropping the superuser privilege
	dropPriviledges(ppfmt)

	// printing the current privileges
	printPriviledges(ppfmt)

	// catching SIGINT and SIGTERM
	chanSignal := make(chan os.Signal, 1)
	signal.Notify(chanSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// context
	ctx := context.Background()

	// reading the config
	c, h, s := initConfig(ctx, ppfmt)
	monitor.StartAll(ctx, ppfmt, c.Monitors)

	first := true
mainLoop:
	for {
		next := c.UpdateCron.Next()
		if !first || c.UpdateOnStart {
			if updater.UpdateIPs(ctx, ppfmt, c, s) {
				monitor.SuccessAll(ctx, ppfmt, c.Monitors)
			} else {
				monitor.FailureAll(ctx, ppfmt, c.Monitors)
			}
		} else if first {
			monitor.SuccessAll(ctx, ppfmt, c.Monitors)
		}
		first = false

		if next.IsZero() {
			if c.DeleteOnStop {
				ppfmt.Errorf(pp.EmojiUserError, "No scheduled updates in near future. Deleting all managed records . . .")
				if !updater.ClearIPs(ctx, ppfmt, c, s) {
					monitor.FailureAll(ctx, ppfmt, c.Monitors)
				}
				ppfmt.Noticef(pp.EmojiBye, "Done now. Bye!")
			} else {
				ppfmt.Errorf(pp.EmojiUserError, "No scheduled updates in near future")
				ppfmt.Noticef(pp.EmojiBye, "Bye!")
			}

			monitor.ExitStatusAll(ctx, ppfmt, c.Monitors, 0)
			break mainLoop
		}

		interval := time.Until(next)
		switch {
		case interval < -IntervalLargeGap:
			ppfmt.Infof(pp.EmojiNow, "Checking the IP addresses now (running behind by %v) . . .",
				-interval.Round(IntervalUnit))
		case interval < IntervalUnit:
			ppfmt.Infof(pp.EmojiNow, "Checking the IP addresses now . . .")
		case interval < IntervalLargeGap:
			ppfmt.Infof(pp.EmojiNow, "Checking the IP addresses in less than %v . . .", IntervalLargeGap)
		default:
			ppfmt.Infof(pp.EmojiAlarm, "Checking the IP addresses in about %v . . .", interval.Round(IntervalUnit))
		}

		sig, ok := signalWait(chanSignal, interval)
		if !ok {
			continue mainLoop
		}
		switch sig.(syscall.Signal) { //nolint:forcetypeassert
		case syscall.SIGHUP:
			ppfmt.Noticef(pp.EmojiSignal, "Caught signal: %v", sig)
			h.FlushCache()

			ppfmt.Noticef(pp.EmojiRepeatOnce, "Restarting . . .")
			c, h, s = initConfig(ctx, ppfmt)
			continue mainLoop

		case syscall.SIGINT, syscall.SIGTERM:
			if c.DeleteOnStop {
				ppfmt.Noticef(pp.EmojiSignal, "Caught signal: %v. Deleting all managed records . . .", sig)
				if !updater.ClearIPs(ctx, ppfmt, c, s) {
					monitor.FailureAll(ctx, ppfmt, c.Monitors)
				}
				ppfmt.Noticef(pp.EmojiBye, "Done now. Bye!")
			} else {
				ppfmt.Noticef(pp.EmojiSignal, "Caught signal: %v", sig)
				ppfmt.Noticef(pp.EmojiBye, "Bye!")
			}

			monitor.ExitStatusAll(ctx, ppfmt, c.Monitors, 0)
			break mainLoop

		default:
			ppfmt.Noticef(pp.EmojiSignal, "Caught and ignored unexpected signal: %v", sig)
			continue mainLoop
		}
	}
}
