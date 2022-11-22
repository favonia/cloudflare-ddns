package main

import (
	"context"
	"fmt"
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

func formatName() string {
	if Version == "" {
		return "Cloudflare DDNS"
	}
	return fmt.Sprintf("Cloudflare DDNS (%s)", Version)
}

func initConfig(ctx context.Context, ppfmt pp.PP) (*config.Config, api.Handle, setter.Setter) {
	c := config.Default()
	bye := func() {
		// Usually, this is called only after initConfig,
		// but we are exiting early.
		monitor.StartAll(ctx, ppfmt, c.Monitors, formatName())

		ppfmt.Noticef(pp.EmojiBye, "Bye!")
		monitor.ExitStatusAll(ctx, ppfmt, c.Monitors, 1, "configuration errors")
		os.Exit(1)
	}

	// Read the config
	if !c.ReadEnv(ppfmt) || !c.NormalizeDomains(ppfmt) {
		bye()
	}

	// Print the config
	c.Print(ppfmt)

	// Get the handler
	h, ok := c.Auth.New(ctx, ppfmt, c.CacheExpiration)
	if !ok {
		bye()
	}

	// Get the setter
	s, ok := setter.New(ppfmt, h)
	if !ok {
		bye()
	}

	return c, h, s
}

func stopUpdating(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) {
	if c.DeleteOnStop {
		ppfmt.Noticef(pp.EmojiClearRecord, "Deleting all managed records . . .")
		if ok, msg := updater.ClearIPs(ctx, ppfmt, c, s); ok {
			monitor.LogAll(ctx, ppfmt, c.Monitors, msg)
		} else {
			monitor.FailureAll(ctx, ppfmt, c.Monitors, msg)
		}
	}
}

func main() { //nolint:funlen
	ppfmt := pp.New(os.Stdout)
	if !config.ReadEmoji("EMOJI", &ppfmt) || !config.ReadQuiet("QUIET", &ppfmt) {
		ppfmt.Noticef(pp.EmojiUserError, "Bye!")
		return
	}
	if !ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Noticef(pp.EmojiMute, "Quiet mode enabled")
	}

	ppfmt.Noticef(pp.EmojiStar, formatName())

	// Drop the superuser privilege
	dropPriviledges(ppfmt)

	// Print the current privileges
	printPriviledges(ppfmt)

	// Catch SIGINT and SIGTERM
	chanSignal := make(chan os.Signal, 1)
	signal.Notify(chanSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Get the context
	ctx := context.Background()

	// Read the config and get the handler and the setter
	c, h, s := initConfig(ctx, ppfmt)

	// Start the tool now
	monitor.StartAll(ctx, ppfmt, c.Monitors, formatName())

	first := true
mainLoop:
	for {
		// The next time to run the updater.
		// This is called before running the updater so that the timer would not be delayed by the updating.
		next := c.UpdateCron.Next()

		// Update the IP
		if !first || c.UpdateOnStart {
			if ok, msg := updater.UpdateIPs(ctx, ppfmt, c, s); ok {
				monitor.SuccessAll(ctx, ppfmt, c.Monitors, msg)
			} else {
				monitor.FailureAll(ctx, ppfmt, c.Monitors, msg)
			}
		} else {
			monitor.SuccessAll(ctx, ppfmt, c.Monitors, "")
		}
		first = false

		// Maybe there's nothing scheduled in near future?
		if next.IsZero() {
			ppfmt.Errorf(pp.EmojiUserError, "No scheduled updates in near future")
			stopUpdating(ctx, ppfmt, c, s)
			ppfmt.Noticef(pp.EmojiBye, "Bye!")
			monitor.ExitStatusAll(ctx, ppfmt, c.Monitors, 0, "Not scheduled updates")
			break mainLoop
		}

		// Display the remaining time interval
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

		// Wait for the next signal or the alarm, whichever comes first
		sig, ok := signalWait(chanSignal, interval)
		if !ok {
			// The alarm comes first
			continue mainLoop
		}
		switch sig.(syscall.Signal) { //nolint:forcetypeassert
		case syscall.SIGHUP:
			ppfmt.Noticef(pp.EmojiSignal, "Caught signal: %v", sig)
			h.FlushCache()

			ppfmt.Noticef(pp.EmojiRepeatOnce, "Restarting . . .")
			c, h, s = initConfig(ctx, ppfmt)
			monitor.LogAll(ctx, ppfmt, c.Monitors, "Restarted")
			continue mainLoop

		case syscall.SIGINT, syscall.SIGTERM:
			ppfmt.Noticef(pp.EmojiSignal, "Caught signal: %v", sig)
			stopUpdating(ctx, ppfmt, c, s)
			ppfmt.Noticef(pp.EmojiBye, "Bye!")
			monitor.ExitStatusAll(ctx, ppfmt, c.Monitors, 0, fmt.Sprintf("Signal: %v", sig))
			break mainLoop

		default:
			ppfmt.Noticef(pp.EmojiSignal, "Caught and ignored unexpected signal: %v", sig)
			continue mainLoop
		}
	}
}
