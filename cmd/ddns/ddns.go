// Package main is the entry point of the Cloudflare DDNS updater.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
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

// stopUpdating reports final cleanup and returns its result without waiting
// for asynchronous operations when their result cannot guide further cleanup.
func stopUpdating(
	ctx context.Context, ppfmt pp.PP,
	lifecycleConfig *config.LifecycleConfig, updateConfig *config.UpdateConfig,
	hb heartbeat.Heartbeat, nt notifier.Notifier,
	s setter.Setter,
) int {
	if lifecycleConfig.DeleteOnStop {
		// Prefer a shorter shutdown over confirming the final item-deletion
		// outcome: there is no further fallback, and waiting risks exhausting
		// the container stop grace period. See the lifecycle design note.
		msg := updater.FinalDeleteIPs(ctx, ppfmt, updateConfig, s, api.CleanupAllowAsync)
		code := operationExitCode(msg)
		hb.Log(ctx, ppfmt, msg.HeartbeatMessage)
		nt.Send(ctx, ppfmt, msg.Notification())
		return code
	}
	return 0
}

// runOnceCleanup confirms cleanup using workCtx and reports the outcome using
// reportCtx, which remains usable if workCtx is canceled. Startup has validated
// that every in-scope provider is static.empty. Reporter delivery does not
// change the returned work result.
func runOnceCleanup(workCtx, reportCtx context.Context, ppfmt pp.PP,
	updateConfig *config.UpdateConfig, hb heartbeat.Heartbeat, nt notifier.Notifier, s setter.Setter,
) int {
	msg := updater.FinalDeleteIPs(workCtx, ppfmt, updateConfig, s, api.CleanupWait)
	code := operationExitCode(msg)
	hb.Ping(reportCtx, ppfmt, msg.HeartbeatMessage)
	nt.Send(reportCtx, ppfmt, msg.Notification())
	return code
}

// runOnceUpdate returns the update result independently of later signals or
// reporter delivery. reportCtx remains usable when workCtx is canceled.
func runOnceUpdate(workCtx, reportCtx context.Context, ppfmt pp.PP,
	updateConfig *config.UpdateConfig, hb heartbeat.Heartbeat, nt notifier.Notifier, s setter.Setter,
) int {
	msg := updater.UpdateIPs(workCtx, ppfmt, updateConfig, s)
	code := operationExitCode(msg)
	hb.Ping(reportCtx, ppfmt, msg.HeartbeatMessage)
	nt.Send(reportCtx, ppfmt, msg.Notification())
	return code
}

func operationExitCode(msg updater.Message) int {
	if msg.Failed() {
		return 1
	}
	return 0
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
		nt.Send(ctx, ppfmt, startupFailureNotification())
		ppfmt.Infof(pp.EmojiBye, "Bye!")
		return 1
	}
	// We only needs lifecycleConfig and updateConfig from now on, and builtConfig should not be used.
	lifecycleConfig := builtConfig.Lifecycle
	updateConfig := builtConfig.Update

	if lifecycleConfig.UpdateCron == nil {
		ppfmt.BlankLineIfVerbose()
		var code int
		if lifecycleConfig.DeleteOnStop {
			code = runOnceCleanup(ctxWithSignals, ctx, ppfmt, updateConfig, hb, nt, s)
		} else {
			code = runOnceUpdate(ctxWithSignals, ctx, ppfmt, updateConfig, hb, nt, s)
		}
		ppfmt.Infof(pp.EmojiBye, "Bye!")
		return code
	}

	nt.Send(ctx, ppfmt, startupNotification())

	// Without the following line, the quiet mode can be too quiet, and some system (Portainer)
	// is not happy with completely empty log. As a workaround, we will print a Notice here.
	// See GitHub issue #426.
	// One-shot execution returns above, preserving its quiet-mode behavior.
	if !ppfmt.IsShowing(pp.Verbose) {
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
			nt.Send(ctx, ppfmt, msg.Notification())
		}

		if ctxWithSignals.Err() != nil {
			goto signaled
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
			nt.Send(ctx, ppfmt, schedulingFailureNotification(
				cron.DescribeSchedule(lifecycleConfig.UpdateCron)))
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			return 1
		}

		// Display the remaining time interval
		cron.PrintCountdown(ppfmt, "Checking the IP addresses", time.Now(), next)

	signaled:
		// Wait for the next signal or the alarm, whichever comes first
		if sig.WaitForSignalsUntil(ppfmt, next) {
			code := stopUpdating(ctx, ppfmt, lifecycleConfig, updateConfig, hb, nt, s)
			hb.Exit(ctx, ppfmt, "Stopped")
			nt.Send(ctx, ppfmt, shutdownNotification())
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			return code
		}
	} // mainLoop
}
