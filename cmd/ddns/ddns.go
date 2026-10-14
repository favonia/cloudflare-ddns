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

// stopUpdating returns final cleanup evidence, or nil when cleanup is disabled.
// It logs the cleanup heartbeat; the caller owns lifecycle notification composition.
func stopUpdating(
	ctx context.Context, ppfmt pp.PP,
	lifecycleConfig *config.LifecycleConfig, updateConfig *config.UpdateConfig,
	hb heartbeat.Heartbeat,
	s setter.Setter,
) *updater.Message {
	if lifecycleConfig.DeleteOnStop {
		// Prefer a shorter shutdown over confirming the final item-deletion
		// outcome: there is no further fallback, and waiting risks exhausting
		// the container stop grace period. See the lifecycle design note.
		msg := updater.FinalDeleteIPs(ctx, ppfmt, updateConfig, s, api.CleanupAllowAsync)
		hb.Log(ctx, ppfmt, msg.HeartbeatMessage)
		return &msg
	}
	return nil
}

// runOnceCleanup confirms cleanup using workCtx and reports the outcome using
// reportCtx, which remains usable if workCtx is canceled. Startup has validated
// that every in-scope provider is static.empty. Reporter delivery does not
// change the returned success flag.
func runOnceCleanup(workCtx, reportCtx context.Context, ppfmt pp.PP,
	updateConfig *config.UpdateConfig, hb heartbeat.Heartbeat, nt notifier.Notifier, s setter.Setter,
) bool {
	msg := updater.FinalDeleteIPs(workCtx, ppfmt, updateConfig, s, api.CleanupWait)
	ok := !msg.Failed()
	hb.Ping(reportCtx, ppfmt, msg.HeartbeatMessage)
	state := updater.NewReportingState(false)
	lifecycle := updater.LifecycleContext{Kind: updater.LifecycleNone, Schedule: ""}
	submitNotification(reportCtx, ppfmt, nt, state, updater.Compose(state.Assess(msg), lifecycle))
	return ok
}

// runOnceUpdate reports whether the update succeeded, independently of later signals or
// reporter delivery. reportCtx remains usable when workCtx is canceled.
func runOnceUpdate(workCtx, reportCtx context.Context, ppfmt pp.PP,
	updateConfig *config.UpdateConfig, hb heartbeat.Heartbeat, nt notifier.Notifier, s setter.Setter,
) bool {
	msg := updater.UpdateIPs(workCtx, ppfmt, updateConfig, s)
	ok := !msg.Failed()
	hb.Ping(reportCtx, ppfmt, msg.HeartbeatMessage)
	state := updater.NewReportingState(false)
	lifecycle := updater.LifecycleContext{Kind: updater.LifecycleNone, Schedule: ""}
	submitNotification(reportCtx, ppfmt, nt, state, updater.Compose(state.Assess(msg), lifecycle))
	return ok
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
		var ok bool
		if lifecycleConfig.DeleteOnStop {
			ok = runOnceCleanup(ctxWithSignals, ctx, ppfmt, updateConfig, hb, nt, s)
		} else {
			ok = runOnceUpdate(ctxWithSignals, ctx, ppfmt, updateConfig, hb, nt, s)
		}
		ppfmt.Infof(pp.EmojiBye, "Bye!")
		if !ok {
			return 1
		}
		return 0
	}

	return runScheduled(ctxWithSignals, ctx, ppfmt, lifecycleConfig, updateConfig, hb, nt, s, sig)
}

// signalWaiter supplies the existing signal-or-deadline wait contract.
type signalWaiter interface {
	WaitForSignalsUntil(ppfmt pp.PP, deadline time.Time) bool
}

// runScheduled owns scheduled work and notification state. reportCtx must remain
// usable after workCtx cancellation; waiter reports whether shutdown was requested.
func runScheduled(workCtx, reportCtx context.Context, ppfmt pp.PP,
	lifecycleConfig *config.LifecycleConfig, updateConfig *config.UpdateConfig,
	hb heartbeat.Heartbeat, nt notifier.Notifier, s setter.Setter, sig signalWaiter,
) int {
	ctx, ctxWithSignals := reportCtx, workCtx
	state := updater.NewReportingState(true)
	if !lifecycleConfig.UpdateOnStart {
		lifecycle := updater.LifecycleContext{Kind: updater.LifecycleStartup, Schedule: ""}
		submitNotification(ctx, ppfmt, nt, state, updater.Compose(updater.Assessment{}, lifecycle))
	}

	// Keep scheduled runs from producing an empty log in quiet mode, which
	// causes problems for Portainer. See GitHub issue #426.
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
			lifecycle := updater.LifecycleContext{Kind: updater.LifecycleNone, Schedule: ""}
			if first {
				lifecycle.Kind = updater.LifecycleStartup
			}
			submitNotification(ctx, ppfmt, nt, state, updater.Compose(state.Assess(msg), lifecycle))
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
			cleanup := stopUpdating(ctx, ppfmt, lifecycleConfig, updateConfig, hb, s)
			hb.Ping(ctx, ppfmt, heartbeat.NewMessagef(false, "No scheduled updates"))
			lifecycle := updater.LifecycleContext{
				Kind: updater.LifecycleSchedulingFailure, Schedule: cron.DescribeSchedule(lifecycleConfig.UpdateCron),
			}
			submitNotification(ctx, ppfmt, nt, state, updater.Compose(assessCleanup(state, cleanup), lifecycle))
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			return 1
		}

		// Display the remaining time interval
		cron.PrintCountdown(ppfmt, "Checking the IP addresses", time.Now(), next)

	signaled:
		// Wait for the next signal or the alarm, whichever comes first
		if sig.WaitForSignalsUntil(ppfmt, next) {
			cleanup := stopUpdating(ctx, ppfmt, lifecycleConfig, updateConfig, hb, s)
			hb.Exit(ctx, ppfmt, "Stopped")
			lifecycle := updater.LifecycleContext{Kind: updater.LifecycleShutdown, Schedule: ""}
			submitNotification(ctx, ppfmt, nt, state, updater.Compose(assessCleanup(state, cleanup), lifecycle))
			ppfmt.Infof(pp.EmojiBye, "Bye!")
			if cleanup != nil && cleanup.Failed() {
				return 1
			}
			return 0
		}
	} // mainLoop
}

// assessCleanup preserves lifecycle-only reporting when cleanup is disabled.
func assessCleanup(state *updater.ReportingState, cleanup *updater.Message) updater.Assessment {
	if cleanup == nil {
		return updater.Assessment{}
	}
	return state.Assess(*cleanup)
}
