// Package main is the entry point of the Cloudflare DDNS updater.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
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

// initConfig reads and builds updater config, prints the resulting settings,
// and constructs the API handle and setter.
//
// It does not set up output formatting or reporter services; those are created
// earlier in bootstrap and passed in so that config printing and later startup
// failures use the same heartbeat/notifier instances.
func initConfig(ppfmt pp.PP, hb heartbeat.Heartbeat, nt notifier.Notifier) (*config.BuiltConfig, setter.Setter, bool) {
	raw := config.DefaultRaw()

	// Read and build the config.
	if !raw.ReadEnv(ppfmt) {
		return nil, nil, false
	}
	builtConfig, ok := raw.BuildConfig(ppfmt)
	if !ok {
		return nil, nil, false
	}

	// Print the config.
	config.Print(ppfmt, builtConfig, hb, nt)

	// Get the handle.
	h, ok := builtConfig.Handle.Auth.New(ppfmt, builtConfig.Handle.Options)
	if !ok {
		return builtConfig, nil, false
	}

	// Get the setter.
	s := setter.New(ppfmt, h)

	return builtConfig, s, true
}

func stopUpdating(
	ctx context.Context, ppfmt pp.PP,
	lifecycleConfig *config.LifecycleConfig, updateConfig *config.UpdateConfig,
	hb heartbeat.Heartbeat, nt notifier.Notifier,
	s setter.Setter,
) {
	if lifecycleConfig.DeleteOnStop {
		msg := updater.FinalDeleteIPs(ctx, ppfmt, updateConfig, s)
		hb.Log(ctx, ppfmt, msg.HeartbeatMessage)
		nt.Send(ctx, ppfmt, msg.NotifierMessage)
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

	// Set up pretty printer. SetupPP intentionally still returns a usable
	// printer on failure so this top-level path can report a final message
	// after the concrete parse error.
	ppfmt, ok := config.SetupPP(os.Stdout)
	if !ok {
		ppfmt.Infof(pp.EmojiBye, "Bye!")
		return 1
	}

	// Show the name and the version of the updater
	ppfmt.Infof(pp.EmojiStar, "%s", formatName())

	// Warn about root privileges
	config.CheckRoot(ppfmt)

	// Set up reporting services before reading the updater config so startup
	// failures during config/handle/setter setup can still be reported through
	// the same heartbeat/notifier instances used after startup.
	hb, nt, reportersOK := config.SetupReporters(ppfmt)
	if !reportersOK {
		ppfmt.Infof(pp.EmojiBye, "Bye!")
		return 1
	}

	// Read the config and get the handle and the setter.
	builtConfig, s, configOK := initConfig(ppfmt, hb, nt)
	// Start heartbeats regardless of whether initConfig succeeded.
	hb.Start(ctx, ppfmt, formatName())
	// Bail out now if initConfig failed
	if !configOK {
		hb.Ping(ctx, ppfmt, heartbeat.NewMessagef(false, "Configuration errors"))
		nt.Send(ctx, ppfmt, notifier.NewMessagef(
			"Cloudflare DDNS was misconfigured and could not start. Please check the logs for details."))
		ppfmt.Infof(pp.EmojiBye, "Bye!")
		return 1
	}
	builtConfig.Handle.Auth.CheckUsability(ctxWithSignals, ppfmt)

	// We only needs lifecycleConfig and updateConfig from now on, and builtConfig should not be used.
	lifecycleConfig := builtConfig.Lifecycle
	updateConfig := builtConfig.Update

	// If UPDATE_CRON is not `@once` (not single-run mode), then send a notification to signal the start.
	if lifecycleConfig.UpdateCron != nil {
		nt.Send(ctx, ppfmt, notifier.NewMessagef("Cloudflare DDNS has started."))
	}

	// Without the following line, the quiet mode can be too quiet, and some system (Portainer)
	// is not happy with completely empty log. As a workaround, we will print a Notice here.
	// See GitHub issue #426.
	//
	// We still want to keep the quiet mode extremely quiet for the single-run mode (UPDATE_CRON=@once),
	// hence we are checking whether cron is enabled or not. (The single-run mode is defined as
	// having the internal cron disabled.)
	if lifecycleConfig.UpdateCron != nil && !ppfmt.IsShowing(pp.Verbose) {
		ppfmt.Noticef(pp.EmojiMute, "Quiet mode enabled")
	}

	first := true
	for {
		// The next time to run the updater.
		// This is called before running the updater so that the timer would not be delayed by the updating.
		next := cron.Next(lifecycleConfig.UpdateCron)

		// Update the IP addresses
		if first && !lifecycleConfig.UpdateOnStart {
			hb.Ping(ctx, ppfmt, heartbeat.NewMessagef(true, "Started (no updates performed yet)"))
		} else {
			// Improve readability of the logging by separating each round of checks with blank lines.
			ppfmt.BlankLineIfVerbose()

			msg := updater.UpdateIPs(ctxWithSignals, ppfmt, updateConfig, s)
			hb.Ping(ctx, ppfmt, msg.HeartbeatMessage)
			nt.Send(ctx, ppfmt, msg.NotifierMessage)
		}

		if ctxWithSignals.Err() != nil {
			goto signaled
		}

		// Check if cron was disabled
		if lifecycleConfig.UpdateCron == nil {
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			return 0
		}

		first = false

		// If there's nothing scheduled in the near future
		if next.IsZero() {
			ppfmt.Noticef(pp.EmojiUserError,
				"No scheduled updates in the near future; consider changing UPDATE_CRON=%s",
				cron.DescribeSchedule(lifecycleConfig.UpdateCron),
			)
			stopUpdating(ctx, ppfmt, lifecycleConfig, updateConfig, hb, nt, s)
			hb.Ping(ctx, ppfmt, heartbeat.NewMessagef(false, "No scheduled updates"))
			nt.Send(ctx, ppfmt,
				notifier.NewMessagef(
					"Cloudflare DDNS stopped because no updates are scheduled in the near future. "+
						"Consider changing the value of UPDATE_CRON (%s).",
					cron.DescribeSchedule(lifecycleConfig.UpdateCron),
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
			stopUpdating(ctx, ppfmt, lifecycleConfig, updateConfig, hb, nt, s)
			hb.Exit(ctx, ppfmt, "Stopped")
			if lifecycleConfig.UpdateCron != nil {
				nt.Send(ctx, ppfmt, notifier.NewMessagef("Cloudflare DDNS has stopped."))
			}
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			return 0
		}
	} // mainLoop
}
