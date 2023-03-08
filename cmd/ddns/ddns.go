// Package main is the entry point of the Cloudflare DDNS updater.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/droproot"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
	"github.com/favonia/cloudflare-ddns/internal/signal"
	"github.com/favonia/cloudflare-ddns/internal/updater"
)

// Version is the version of the updater that will be shown in the output.
// This is to be overwritten by the linker argument -X main.Version=version.
var Version string //nolint:gochecknoglobals

func formatName() string {
	if Version == "" {
		return "Cloudflare DDNS"
	}
	return fmt.Sprintf("Cloudflare DDNS (%s)", Version)
}

func initConfig(ctx context.Context, ppfmt pp.PP) (*config.Config, setter.Setter, bool) {
	c := config.Default()

	// Read the config
	if !c.ReadEnv(ppfmt) || !c.NormalizeDomains(ppfmt) {
		return c, nil, false
	}

	// Print the config
	c.Print(ppfmt)

	// Get the handler
	h, ok := c.Auth.New(ctx, ppfmt, c.CacheExpiration)
	if !ok {
		return c, nil, false
	}

	// Get the setter
	s, ok := setter.New(ppfmt, h)
	if !ok {
		return c, nil, false
	}

	return c, s, true
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

func main() {
	os.Exit(realMain())
}

func realMain() int { //nolint:funlen
	ppfmt := pp.New(os.Stdout)
	if !config.ReadEmoji("EMOJI", &ppfmt) || !config.ReadQuiet("QUIET", &ppfmt) {
		ppfmt.Noticef(pp.EmojiUserError, "Bye!")
		return 1
	}
	if !ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Noticef(pp.EmojiMute, "Quiet mode enabled")
	}

	// Show the name and the version of the updater
	ppfmt.Noticef(pp.EmojiStar, formatName())

	// Drop the superuser privilege
	droproot.DropPriviledges(ppfmt)
	droproot.PrintPriviledges(ppfmt)

	// Catch signals SIGINT and SIGTERM
	sig := signal.Setup()

	// Get the contexts
	ctx := context.Background()
	ctxWithSignals, _ := signal.NotifyContext(ctx)

	// Read the config and get the handler and the setter
	c, s, configOk := initConfig(ctx, ppfmt)
	// Ping the monitor regardless of whether initConfig succeeded
	c.Monitor.Start(ctx, ppfmt, formatName())
	// Bail out now if initConfig failed
	if !configOk {
		c.Monitor.ExitStatus(ctx, ppfmt, 1, "Config errors")
		ppfmt.Noticef(pp.EmojiBye, "Bye!")
		return 1
	}

	first := true
	for {
		// The next time to run the updater.
		// This is called before running the updater so that the timer would not be delayed by the updating.
		next := c.UpdateCron.Next()

		// Update the IP addresses
		if first && !c.UpdateOnStart {
			c.Monitor.Success(ctx, ppfmt, "Started (no action)")
		} else {
			if ok, msg := updater.UpdateIPs(ctxWithSignals, ppfmt, c, s); ok {
				c.Monitor.Success(ctx, ppfmt, msg)
			} else {
				c.Monitor.Failure(ctx, ppfmt, msg)
			}
		}
		first = false

		// Maybe the cron was disabled?
		if !c.UpdateCron.IsEnabled() {
			ppfmt.Noticef(pp.EmojiBye, "Bye!")
			return 0
		}

		// Maybe there's nothing scheduled in near future?
		if next.IsZero() {
			ppfmt.Errorf(pp.EmojiUserError, "No scheduled updates in near future")
			stopUpdating(ctx, ppfmt, c, s)
			c.Monitor.ExitStatus(ctx, ppfmt, 1, "No scheduled updates")
			ppfmt.Noticef(pp.EmojiBye, "Bye!")
			return 1
		}

		// Display the remaining time interval
		interval := time.Until(next)
		cron.PrintCountdown(ppfmt, "Checking the IP addresses", interval)

		// Wait for the next signal or the alarm, whichever comes first
		if !sig.Sleep(ppfmt, interval) {
			stopUpdating(ctx, ppfmt, c, s)
			c.Monitor.ExitStatus(ctx, ppfmt, 0, "Terminated")
			ppfmt.Noticef(pp.EmojiBye, "Bye!")
			return 0
		}
	} // mainLoop
}
