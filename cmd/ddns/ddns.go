// Package main is the entry point of the Cloudflare DDNS updater.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
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

func initConfig(ppfmt pp.PP) (*config.Config, setter.Setter, bool) {
	c := config.Default()

	// Read the config
	if !c.ReadEnv(ppfmt) || !c.Normalize(ppfmt) {
		return c, nil, false
	}

	// Print the config
	c.Print(ppfmt)

	// Get the handler
	h, ok := c.Auth.New(ppfmt, c.CacheExpiration)
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
		msg := updater.DeleteIPs(ctx, ppfmt, c, s)
		c.Monitor.Log(ctx, ppfmt, msg.MonitorMessage)
		c.Notifier.Send(ctx, ppfmt, msg.NotifierMessage)
	}
}

func main() {
	// This is to make os.Exit work with defer
	os.Exit(realMain())
}

func realMain() int {
	// Get the contexts and start catching SIGINT and SIGTERM
	ctx := context.Background()
	sig := signal.Setup()
	ctxWithSignals, _ := signal.NotifyContext(ctx)

	// Set up pretty printer
	ppfmt, ok := config.SetupPP(os.Stdout)
	if !ok {
		ppfmt.Infof(pp.EmojiUserError, "Bye!")
		return 1
	}

	// Show the name and the version of the updater
	ppfmt.Infof(pp.EmojiStar, "%s", formatName())

	// Warn about root privileges
	config.CheckRoot(ppfmt)

	// Read the config and get the handler and the setter
	c, s, configOK := initConfig(ppfmt)
	// Ping monitors regardless of whether initConfig succeeded
	c.Monitor.Start(ctx, ppfmt, formatName())
	// Bail out now if initConfig failed
	if !configOK {
		c.Monitor.Ping(ctx, ppfmt, monitor.NewMessagef(false, "Configuration errors"))
		c.Notifier.Send(ctx, ppfmt, notifier.NewMessagef(
			"Cloudflare DDNS was misconfigured and could not start. Please check the logging for details."))
		ppfmt.Infof(pp.EmojiBye, "Bye!")
		return 1
	}
	// If UPDATE_CRON is not `@once` (not single-run mode), then send a notification to signal the start.
	if c.UpdateCron != nil {
		c.Notifier.Send(ctx, ppfmt, notifier.NewMessagef("Started running Cloudflare DDNS."))
	}

	// Without the following line, the quiet mode can be too quiet, and some system (Portainer)
	// is not happy with completely empty log. As a workaround, we will print a Notice here.
	// See GitHub issue #426.
	//
	// We still want to keep the quiet mode extremely quiet for the single-run mode (UPDATE_CRON=@once),
	// hence we are checking whether cron is enabled or not. (The single-run mode is defined as
	// having the internal cron disabled.)
	if c.UpdateCron != nil && !ppfmt.IsShowing(pp.Verbose) {
		ppfmt.Noticef(pp.EmojiMute, "Quiet mode enabled")
	}

	first := true
	for {
		// The next time to run the updater.
		// This is called before running the updater so that the timer would not be delayed by the updating.
		next := cron.Next(c.UpdateCron)

		// Update the IP addresses
		if first && !c.UpdateOnStart {
			c.Monitor.Ping(ctx, ppfmt, monitor.NewMessagef(true, "Started (no action)"))
		} else {
			msg := updater.UpdateIPs(ctxWithSignals, ppfmt, c, s)
			c.Monitor.Ping(ctx, ppfmt, msg.MonitorMessage)
			c.Notifier.Send(ctx, ppfmt, msg.NotifierMessage)
		}

		if ctxWithSignals.Err() != nil {
			goto signaled
		}

		// Check if cron was disabled
		if c.UpdateCron == nil {
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			return 0
		}

		first = false

		// If there's nothing scheduled in near future
		if next.IsZero() {
			ppfmt.Noticef(pp.EmojiUserError,
				"No scheduled updates in near future; consider changing UPDATE_CRON=%s",
				cron.DescribeSchedule(c.UpdateCron),
			)
			stopUpdating(ctx, ppfmt, c, s)
			c.Monitor.Ping(ctx, ppfmt, monitor.NewMessagef(false, "No scheduled updates"))
			c.Notifier.Send(ctx, ppfmt,
				notifier.NewMessagef(
					"Cloudflare DDNS stopped because there are no scheduled updates in near future. "+
						"Consider changing the value of UPDATE_CRON (%s).",
					cron.DescribeSchedule(c.UpdateCron),
				),
			)
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			return 1
		}

		// Display the remaining time interval
		cron.PrintCountdown(ppfmt, "Checking the IP addresses", time.Now(), next)

	signaled:
		// Wait for the next signal or the alarm, whichever comes first
		if sig.WaitForSignalsUntil(ppfmt, next) {
			stopUpdating(ctx, ppfmt, c, s)
			c.Monitor.Exit(ctx, ppfmt, "Stopped")
			if c.UpdateCron != nil {
				c.Notifier.Send(ctx, ppfmt, notifier.NewMessagef("Stopped running Cloudflare DDNS."))
			}
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			return 0
		}
	} // mainLoop
}
