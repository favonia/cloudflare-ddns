// Package main is the entry point of the Cloudflare DDNS updater.
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
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/droproot"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
	"github.com/favonia/cloudflare-ddns/internal/updater"
)

const (
	intervalUnit     = time.Second
	intervalLargeGap = time.Second * 10
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

// Version is the version of the updater that will be shown in the output.
// This is to be overwritten by the linker argument -X main.Version=version.
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
		c.Monitor.Start(ctx, ppfmt, formatName())

		ppfmt.Noticef(pp.EmojiBye, "Bye!")
		c.Monitor.ExitStatus(ctx, ppfmt, 1, "configuration errors")
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
			c.Monitor.Log(ctx, ppfmt, msg)
		} else {
			c.Monitor.Failure(ctx, ppfmt, msg)
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
	droproot.DropPriviledges(ppfmt)

	// Print the current privileges
	droproot.PrintPriviledges(ppfmt)

	// Catch SIGINT and SIGTERM
	chanSignal := make(chan os.Signal, 1)
	signal.Notify(chanSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Get the context
	ctx := context.Background()

	// Read the config and get the handler and the setter
	c, h, s := initConfig(ctx, ppfmt)

	// Start the tool now
	c.Monitor.Start(ctx, ppfmt, formatName())

	first := true
mainLoop:
	for {
		// The next time to run the updater.
		// This is called before running the updater so that the timer would not be delayed by the updating.
		next := c.UpdateCron.Next()

		// Update the IP
		if !first || c.UpdateOnStart {
			if ok, msg := updater.UpdateIPs(ctx, ppfmt, c, s); ok {
				c.Monitor.Success(ctx, ppfmt, msg)
			} else {
				c.Monitor.Failure(ctx, ppfmt, msg)
			}
		} else {
			c.Monitor.Success(ctx, ppfmt, "")
		}
		first = false

		// Maybe there's nothing scheduled in near future?
		if next.IsZero() {
			ppfmt.Errorf(pp.EmojiUserError, "No scheduled updates in near future")
			stopUpdating(ctx, ppfmt, c, s)
			ppfmt.Noticef(pp.EmojiBye, "Bye!")
			c.Monitor.ExitStatus(ctx, ppfmt, 0, "Not scheduled updates")
			break mainLoop
		}

		// Display the remaining time interval
		interval := time.Until(next)
		cron.PrintCountdown(ppfmt, "Checking the IP addresses", interval)

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
			c.Monitor.Log(ctx, ppfmt, "Restarted")
			continue mainLoop

		case syscall.SIGINT, syscall.SIGTERM:
			ppfmt.Noticef(pp.EmojiSignal, "Caught signal: %v", sig)
			stopUpdating(ctx, ppfmt, c, s)
			ppfmt.Noticef(pp.EmojiBye, "Bye!")
			c.Monitor.ExitStatus(ctx, ppfmt, 0, fmt.Sprintf("Signal: %v", sig))
			break mainLoop

		default:
			ppfmt.Noticef(pp.EmojiSignal, "Caught and ignored unexpected signal: %v", sig)
			continue mainLoop
		}
	}
}
