// Package main is the entry point of the Cloudflare DDNS updater.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/droproot"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
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
	if !c.ReadEnv(ppfmt) || !c.NormalizeConfig(ppfmt) || !c.ShouldWeUse1001(ctx, ppfmt) {
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
		resp := updater.DeleteIPs(ctx, ppfmt, c, s)
		monitor.SendResponseAll(ctx, ppfmt, c.Monitors, resp, false)
		notifier.SendResponseAll(ctx, ppfmt, c.Notifiers, resp)
	}
}

func main() {
	// This is to make os.Exit work with defer
	os.Exit(realMain())
}

func realMain() int { //nolint:funlen
	ppfmt := pp.New(os.Stdout)
	if !config.ReadEmoji("EMOJI", &ppfmt) || !config.ReadQuiet("QUIET", &ppfmt) {
		ppfmt.Infof(pp.EmojiUserError, "Bye!")
		return 1
	}

	// Show the name and the version of the updater
	ppfmt.Infof(pp.EmojiStar, formatName())

	// Drop the superuser privilege
	if !droproot.DropPrivileges(ppfmt) {
		ppfmt.Infof(pp.EmojiBye, "Bye!")
		return 1
	}

	// Catch signals SIGINT and SIGTERM
	sig := signal.Setup()

	// Get the contexts
	ctx := context.Background()
	ctxWithSignals, _ := signal.NotifyContext(ctx)

	// Read the config and get the handler and the setter
	c, s, configOk := initConfig(ctx, ppfmt)
	// Ping monitors and notifiers regardless of whether initConfig succeeded
	monitor.StartAll(ctx, ppfmt, c.Monitors, formatName())
	// Bail out now if initConfig failed
	if !configOk {
		monitor.ExitStatusAll(ctx, ppfmt, c.Monitors, 1, "Config errors")
		notifier.SendAll(ctx, ppfmt, c.Notifiers,
			"The configuration has errors. Please check the logging for more details.")
		ppfmt.Infof(pp.EmojiBye, "Bye!")
		return 1
	}
	// If UPDATE_CRON is not `@once` (not single-run mode), then send a notification.
	if c.UpdateCron != nil {
		notifier.SendAll(ctx, ppfmt, c.Notifiers, fmt.Sprintf("Started running %s.", formatName()))
	}

	// Without the following line, the quiet mode can be too quiet, and some system (Portainer)
	// is not happy with completely empty log. As a workaround, we will print a Notice here.
	// See GitHub issue #426.
	//
	// We still want to keep the quiet mode extremely quiet for the single-run mode (UPDATE_CRON=@once),
	// hence we are checking whether cron is enabled or not. (The single-run mode is defined as
	// having the internal cron disabled.)
	if c.UpdateCron != nil && !ppfmt.IsEnabledFor(pp.Verbose) {
		ppfmt.Noticef(pp.EmojiMute, "Quiet mode enabled")
	}

	first := true
	for {
		// The next time to run the updater.
		// This is called before running the updater so that the timer would not be delayed by the updating.
		next := cron.Next(c.UpdateCron)

		// Update the IP addresses
		if first && !c.UpdateOnStart {
			monitor.SuccessAll(ctx, ppfmt, c.Monitors, "Started (no action)")
		} else {
			resp := updater.UpdateIPs(ctxWithSignals, ppfmt, c, s)
			monitor.SendResponseAll(ctx, ppfmt, c.Monitors, resp, true)
			notifier.SendResponseAll(ctx, ppfmt, c.Notifiers, resp)
		}

		// Check if cron was disabled
		if c.UpdateCron == nil {
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			return 0
		}

		first = false

		// If there's nothing scheduled in near future
		if next.IsZero() {
			ppfmt.Errorf(pp.EmojiUserError, "No scheduled updates in near future")
			stopUpdating(ctx, ppfmt, c, s)
			monitor.ExitStatusAll(ctx, ppfmt, c.Monitors, 1, "No scheduled updates")
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			return 1
		}

		// Display the remaining time interval
		cron.PrintCountdown(ppfmt, "Checking the IP addresses", next)

		// Wait for the next signal or the alarm, whichever comes first
		if !sig.SleepUntil(ppfmt, next) {
			stopUpdating(ctx, ppfmt, c, s)
			monitor.ExitStatusAll(ctx, ppfmt, c.Monitors, 0, "Terminated")
			notifier.SendAll(ctx, ppfmt, c.Notifiers, formatName()+" was terminated.")
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			return 0
		}
	} // mainLoop
}
