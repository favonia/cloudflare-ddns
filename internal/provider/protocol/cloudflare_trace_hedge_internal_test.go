package protocol

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

type traceAttemptControl struct {
	started   chan time.Time
	release   chan traceAttemptResult
	canceled  chan struct{}
	allowExit chan struct{}
	exited    chan struct{}
}

type traceCancellationWindowContext struct {
	context.Context //nolint:containedctx // Test-only wrapper intercepts the coordinator's Err observations.

	cancel       context.CancelCauseFunc
	windowStart  time.Time
	windowEnd    time.Time
	target       int64
	observations atomic.Int64
	triggered    atomic.Bool
}

func (ctx *traceCancellationWindowContext) Err() error {
	now := time.Now()
	inWindow := !now.Before(ctx.windowStart) && now.Before(ctx.windowEnd)
	if inWindow && !ctx.triggered.Load() && ctx.observations.Add(1) == ctx.target {
		ctx.triggered.Store(true)
		ctx.cancel(context.Canceled)
	}
	return ctx.Context.Err() //nolint:wrapcheck // Context.Err must preserve its cancellation sentinel.
}

func newTraceAttemptControls(endpoints []string) map[string]*traceAttemptControl {
	controls := make(map[string]*traceAttemptControl, len(endpoints))
	for _, endpoint := range endpoints {
		controls[endpoint] = &traceAttemptControl{
			started:   make(chan time.Time, len(endpoints)),
			release:   make(chan traceAttemptResult, 1),
			canceled:  make(chan struct{}, 1),
			allowExit: make(chan struct{}, 1),
			exited:    make(chan struct{}, 1),
		}
	}
	return controls
}

func traceTestAttemptResult(status traceAttemptStatus) traceAttemptResult {
	return traceAttemptResult{
		status:   status,
		rawData:  NewUnavailableDetectionResult(),
		warnings: nil,
		failure:  traceFailure{}, //nolint:exhaustruct // Tests need only the scheduler status.
	}
}

func controlledTraceAttempt(controls map[string]*traceAttemptControl) traceAttemptFunc {
	return func(ctx context.Context, endpoint string) traceAttemptResult {
		control := controls[endpoint]
		control.started <- time.Now()
		defer func() { control.exited <- struct{}{} }()

		select {
		case result := <-control.release:
			return result
		case <-ctx.Done():
			control.canceled <- struct{}{}
			return traceTestAttemptResult(traceAttemptCanceled)
		}
	}
}

func startCloudflareTraceRun(
	ctx context.Context,
	endpoints []string,
	hedgeDelay time.Duration,
	attempt traceAttemptFunc,
) <-chan traceRunResult {
	done := make(chan traceRunResult, 1)
	go func() {
		done <- runCloudflareTraceAttempts(ctx, endpoints, hedgeDelay, attempt)
	}()
	return done
}

func receiveTraceStart(t *testing.T, control *traceAttemptControl) time.Time {
	t.Helper()
	return <-control.started
}

func requireNoTraceStart(t *testing.T, control *traceAttemptControl) {
	t.Helper()
	select {
	case startedAt := <-control.started:
		t.Fatalf("unexpected trace attempt started at %s", startedAt)
	default:
	}
}

func requireTraceStatuses(t *testing.T, run traceRunResult, statuses ...traceAttemptStatus) {
	t.Helper()
	require.Len(t, run.attempts, len(statuses))
	for index, status := range statuses {
		require.Equal(t, status, run.attempts[index].status, "endpoint index %d", index)
	}
}

func TestCloudflareTraceHedgeDelay(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tests := []struct {
		name     string
		deadline time.Duration
		want     time.Duration
		positive bool
	}{
		{name: "no-deadline", deadline: 0, want: 250 * time.Millisecond, positive: true},
		{name: "default-five-seconds", deadline: 5 * time.Second, want: 250 * time.Millisecond, positive: true},
		{name: "short-two-seconds", deadline: 2 * time.Second, want: 100 * time.Millisecond, positive: true},
		{name: "long-one-minute", deadline: time.Minute, want: 250 * time.Millisecond, positive: true},
		{name: "expired-deadline", deadline: -time.Second, want: 0, positive: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if test.name != "no-deadline" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, now.Add(test.deadline))
				defer cancel()
			}

			got := cloudflareTraceHedgeDelay(ctx, now)
			if test.positive {
				require.Equal(t, test.want, got)
			} else {
				require.LessOrEqual(t, got, time.Duration(0))
			}
		})
	}
}

func TestRunCloudflareTraceAttemptsPrimarySuccessBeforeHedge(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		endpoints := []string{"primary", "second", "third"}
		controls := newTraceAttemptControls(endpoints)
		done := startCloudflareTraceRun(
			context.Background(), endpoints, 100*time.Millisecond, controlledTraceAttempt(controls),
		)

		receiveTraceStart(t, controls["primary"])
		controls["primary"].release <- traceTestAttemptResult(traceAttemptSucceeded)
		run := <-done

		require.Equal(t, 0, run.winnerIndex)
		require.False(t, run.timedOut)
		requireTraceStatuses(t, run,
			traceAttemptSucceeded, traceAttemptUnstarted, traceAttemptUnstarted)
		requireNoTraceStart(t, controls["second"])
		requireNoTraceStart(t, controls["third"])
	})
}

func TestRunCloudflareTraceAttemptsTimerLaunchesHedges(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		endpoints := []string{"primary", "second", "third"}
		controls := newTraceAttemptControls(endpoints)
		delay := 100 * time.Millisecond
		done := startCloudflareTraceRun(
			context.Background(), endpoints, delay, controlledTraceAttempt(controls),
		)

		primaryStart := receiveTraceStart(t, controls["primary"])
		secondStart := receiveTraceStart(t, controls["second"])
		require.Equal(t, delay, secondStart.Sub(primaryStart))
		controls["second"].release <- traceTestAttemptResult(traceAttemptSucceeded)
		run := <-done

		require.Equal(t, 1, run.winnerIndex)
		requireTraceStatuses(t, run,
			traceAttemptCanceled, traceAttemptSucceeded, traceAttemptUnstarted)
		requireNoTraceStart(t, controls["third"])
	})
}

func TestRunCloudflareTraceAttemptsFailureAcceleratesAndRestartsTimer(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		endpoints := []string{"primary", "second", "third"}
		controls := newTraceAttemptControls(endpoints)
		delay := 100 * time.Millisecond
		done := startCloudflareTraceRun(
			context.Background(), endpoints, delay, controlledTraceAttempt(controls),
		)

		primaryStart := receiveTraceStart(t, controls["primary"])
		time.Sleep(25 * time.Millisecond)
		controls["primary"].release <- traceTestAttemptResult(traceAttemptFailed)
		secondStart := receiveTraceStart(t, controls["second"])
		require.Equal(t, 25*time.Millisecond, secondStart.Sub(primaryStart))

		thirdStart := receiveTraceStart(t, controls["third"])
		require.Equal(t, delay, thirdStart.Sub(secondStart))
		controls["third"].release <- traceTestAttemptResult(traceAttemptSucceeded)
		run := <-done

		require.Equal(t, 2, run.winnerIndex)
		requireTraceStatuses(t, run,
			traceAttemptFailed, traceAttemptCanceled, traceAttemptSucceeded)
	})
}

func TestRunCloudflareTraceAttemptsOlderFailureAcceleratesPendingEndpoint(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		endpoints := []string{"primary", "second", "third"}
		controls := newTraceAttemptControls(endpoints)
		delay := 100 * time.Millisecond
		done := startCloudflareTraceRun(
			context.Background(), endpoints, delay, controlledTraceAttempt(controls),
		)

		primaryStart := receiveTraceStart(t, controls["primary"])
		secondStart := receiveTraceStart(t, controls["second"])
		require.Equal(t, delay, secondStart.Sub(primaryStart))
		time.Sleep(25 * time.Millisecond)
		controls["primary"].release <- traceTestAttemptResult(traceAttemptFailed)
		thirdStart := receiveTraceStart(t, controls["third"])
		require.Equal(t, 25*time.Millisecond, thirdStart.Sub(secondStart))

		controls["third"].release <- traceTestAttemptResult(traceAttemptSucceeded)
		run := <-done
		require.Equal(t, 2, run.winnerIndex)
		requireTraceStatuses(t, run,
			traceAttemptFailed, traceAttemptCanceled, traceAttemptSucceeded)
	})
}

func TestRunCloudflareTraceAttemptsLaunchesAllByTwoDelays(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		endpoints := []string{"primary", "second", "third"}
		controls := newTraceAttemptControls(endpoints)
		delay := 100 * time.Millisecond
		done := startCloudflareTraceRun(
			context.Background(), endpoints, delay, controlledTraceAttempt(controls),
		)

		primaryStart := receiveTraceStart(t, controls["primary"])
		secondStart := receiveTraceStart(t, controls["second"])
		thirdStart := receiveTraceStart(t, controls["third"])
		require.Equal(t, delay, secondStart.Sub(primaryStart))
		require.Equal(t, 2*delay, thirdStart.Sub(primaryStart))

		controls["third"].release <- traceTestAttemptResult(traceAttemptSucceeded)
		run := <-done
		require.Equal(t, 2, run.winnerIndex)
		requireTraceStatuses(t, run,
			traceAttemptCanceled, traceAttemptCanceled, traceAttemptSucceeded)
	})
}

func TestRunCloudflareTraceAttemptsNonPositiveDelayLaunchesAll(t *testing.T) {
	t.Parallel()

	for _, delay := range []time.Duration{0, -time.Millisecond} {
		t.Run(delay.String(), func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				endpoints := []string{"primary", "second", "third"}
				controls := newTraceAttemptControls(endpoints)
				done := startCloudflareTraceRun(
					context.Background(), endpoints, delay, controlledTraceAttempt(controls),
				)

				primaryStart := receiveTraceStart(t, controls["primary"])
				secondStart := receiveTraceStart(t, controls["second"])
				thirdStart := receiveTraceStart(t, controls["third"])
				require.Equal(t, primaryStart, secondStart)
				require.Equal(t, primaryStart, thirdStart)

				controls["primary"].release <- traceTestAttemptResult(traceAttemptSucceeded)
				run := <-done
				require.Equal(t, 0, run.winnerIndex)
				requireTraceStatuses(t, run,
					traceAttemptSucceeded, traceAttemptCanceled, traceAttemptCanceled)
			})
		})
	}
}

func TestRunCloudflareTraceAttemptsFirstCoordinatorCompletionWins(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		endpoints := []string{"primary", "second", "third"}
		controls := newTraceAttemptControls(endpoints)
		done := startCloudflareTraceRun(
			context.Background(), endpoints, 0, controlledTraceAttempt(controls),
		)

		receiveTraceStart(t, controls["primary"])
		receiveTraceStart(t, controls["second"])
		receiveTraceStart(t, controls["third"])
		controls["second"].release <- traceTestAttemptResult(traceAttemptSucceeded)
		run := <-done

		require.Equal(t, 1, run.winnerIndex)
		requireTraceStatuses(t, run,
			traceAttemptCanceled, traceAttemptSucceeded, traceAttemptCanceled)
	})
}

func TestRunCloudflareTraceAttemptsWinnerCancelsAndDrainsStartedWorkers(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		endpoints := []string{"primary", "second", "third"}
		controls := newTraceAttemptControls(endpoints)
		attempt := func(ctx context.Context, endpoint string) traceAttemptResult {
			control := controls[endpoint]
			control.started <- time.Now()
			defer func() { control.exited <- struct{}{} }()
			if endpoint == "primary" {
				return <-control.release
			}
			<-ctx.Done()
			control.canceled <- struct{}{}
			<-control.allowExit
			return traceTestAttemptResult(traceAttemptCanceled)
		}
		done := startCloudflareTraceRun(context.Background(), endpoints, 0, attempt)

		receiveTraceStart(t, controls["primary"])
		receiveTraceStart(t, controls["second"])
		receiveTraceStart(t, controls["third"])
		controls["primary"].release <- traceTestAttemptResult(traceAttemptSucceeded)
		<-controls["second"].canceled
		<-controls["third"].canceled
		select {
		case <-done:
			t.Fatal("coordinator returned before canceled workers exited")
		default:
		}

		controls["second"].allowExit <- struct{}{}
		<-controls["second"].exited
		select {
		case <-done:
			t.Fatal("coordinator returned before every canceled worker exited")
		default:
		}
		controls["third"].allowExit <- struct{}{}
		<-controls["third"].exited
		run := <-done

		require.Equal(t, 0, run.winnerIndex)
		requireTraceStatuses(t, run,
			traceAttemptSucceeded, traceAttemptCanceled, traceAttemptCanceled)
		<-controls["primary"].exited
	})
}

func TestRunCloudflareTraceAttemptsParentCancellationStopsNewLaunchesAndDrains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		newContext        func() (context.Context, func())
		cancelImmediately bool
		timedOut          bool
	}{
		{
			name: "deadline-from-err",
			newContext: func() (context.Context, func()) {
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				return ctx, cancel
			},
			cancelImmediately: false,
			timedOut:          true,
		},
		{
			name: "deadline-from-cause",
			newContext: func() (context.Context, func()) {
				ctx, cancel := context.WithCancelCause(context.Background())
				return ctx, func() { cancel(context.DeadlineExceeded) }
			},
			cancelImmediately: true,
			timedOut:          true,
		},
		{
			name: "external-cancellation",
			newContext: func() (context.Context, func()) {
				return context.WithCancel(context.Background())
			},
			cancelImmediately: true,
			timedOut:          false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				endpoints := []string{"primary", "second", "third"}
				controls := newTraceAttemptControls(endpoints)
				ctx, cancel := test.newContext()
				defer cancel()
				attempt := func(ctx context.Context, endpoint string) traceAttemptResult {
					control := controls[endpoint]
					control.started <- time.Now()
					defer func() { control.exited <- struct{}{} }()
					<-ctx.Done()
					control.canceled <- struct{}{}
					<-control.release
					return traceTestAttemptResult(traceAttemptSucceeded)
				}
				done := startCloudflareTraceRun(ctx, endpoints, 100*time.Millisecond, attempt)

				receiveTraceStart(t, controls["primary"])
				if test.cancelImmediately {
					cancel()
				}
				<-controls["primary"].canceled
				requireNoTraceStart(t, controls["second"])
				requireNoTraceStart(t, controls["third"])
				select {
				case <-done:
					t.Fatal("coordinator returned before the started worker exited")
				default:
				}

				controls["primary"].release <- traceTestAttemptResult(traceAttemptSucceeded)
				<-controls["primary"].exited
				run := <-done

				require.Equal(t, -1, run.winnerIndex)
				require.Equal(t, test.timedOut, run.timedOut)
				requireTraceStatuses(t, run,
					traceAttemptSucceeded, traceAttemptUnstarted, traceAttemptUnstarted)
			})
		})
	}
}

func TestRunCloudflareTraceAttemptsCanceledBeforeLaunch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		newContext func() (context.Context, context.CancelFunc)
		timedOut   bool
	}{
		{
			name: "external-cancellation",
			newContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			timedOut: false,
		},
		{
			name: "expired-deadline",
			newContext: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			timedOut: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := test.newContext()
			defer cancel()
			var attempts atomic.Int64
			run := runCloudflareTraceAttempts(
				ctx,
				[]string{"primary", "fallback"},
				100*time.Millisecond,
				func(context.Context, string) traceAttemptResult {
					attempts.Add(1)
					return traceTestAttemptResult(traceAttemptSucceeded)
				},
			)

			// Mutation caught: launching an endpoint after cancellation or losing the timeout distinction.
			require.EqualValues(t, 0, attempts.Load())
			require.Equal(t, -1, run.winnerIndex)
			require.Equal(t, test.timedOut, run.timedOut)
			requireTraceStatuses(t, run, traceAttemptUnstarted, traceAttemptUnstarted)
		})
	}
}

func TestRunCloudflareTraceAttemptsCancellationObservationWindows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		endpoints        []string
		hedgeDelay       time.Duration
		plannedStatus    traceAttemptStatus
		hasPlannedResult bool
		windowStart      time.Duration
		windowEnd        time.Duration
		target           int64
		wantStatuses     []traceAttemptStatus
	}{
		{
			name:             "non-positive-delay-launch-loop",
			endpoints:        []string{"primary", "fallback"},
			hedgeDelay:       0,
			plannedStatus:    traceAttemptUnstarted,
			hasPlannedResult: false,
			windowStart:      0,
			windowEnd:        5 * time.Second,
			target:           2,
			wantStatuses:     []traceAttemptStatus{traceAttemptCanceled, traceAttemptUnstarted},
		},
		{
			name:             "top-of-loop",
			endpoints:        []string{"primary", "fallback"},
			hedgeDelay:       time.Hour,
			plannedStatus:    traceAttemptUnstarted,
			hasPlannedResult: false,
			windowStart:      0,
			windowEnd:        5 * time.Second,
			target:           2,
			wantStatuses:     []traceAttemptStatus{traceAttemptCanceled, traceAttemptUnstarted},
		},
		{
			name:             "after-result",
			endpoints:        []string{"primary"},
			hedgeDelay:       time.Hour,
			plannedStatus:    traceAttemptSucceeded,
			hasPlannedResult: true,
			windowStart:      10 * time.Second,
			windowEnd:        15 * time.Second,
			target:           1,
			wantStatuses:     []traceAttemptStatus{traceAttemptSucceeded},
		},
		{
			name:             "failed-result-launch",
			endpoints:        []string{"primary", "fallback"},
			hedgeDelay:       time.Hour,
			plannedStatus:    traceAttemptFailed,
			hasPlannedResult: true,
			windowStart:      10 * time.Second,
			windowEnd:        15 * time.Second,
			target:           2,
			wantStatuses:     []traceAttemptStatus{traceAttemptFailed, traceAttemptUnstarted},
		},
		{
			name:             "after-final-result",
			endpoints:        []string{"primary"},
			hedgeDelay:       time.Hour,
			plannedStatus:    traceAttemptFailed,
			hasPlannedResult: true,
			windowStart:      10 * time.Second,
			windowEnd:        15 * time.Second,
			target:           2,
			wantStatuses:     []traceAttemptStatus{traceAttemptFailed},
		},
		{
			name:             "timer-launch",
			endpoints:        []string{"primary", "fallback"},
			hedgeDelay:       10 * time.Second,
			plannedStatus:    traceAttemptUnstarted,
			hasPlannedResult: false,
			windowStart:      10 * time.Second,
			windowEnd:        15 * time.Second,
			target:           1,
			wantStatuses:     []traceAttemptStatus{traceAttemptCanceled, traceAttemptUnstarted},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				now := time.Now()
				windowStart := now.Add(test.windowStart)
				windowEnd := now.Add(test.windowEnd)
				deadlineCtx, cancelDeadline := context.WithDeadline(context.Background(), windowEnd)
				parentCtx, cancel := context.WithCancelCause(deadlineCtx)
				probe := &traceCancellationWindowContext{
					Context:      parentCtx,
					cancel:       cancel,
					windowStart:  windowStart,
					windowEnd:    windowEnd,
					target:       test.target,
					observations: atomic.Int64{},
					triggered:    atomic.Bool{},
				}
				cleanup := func() {
					cancel(context.Canceled)
					cancelDeadline()
				}
				defer cleanup()

				var started atomic.Int64
				var exited atomic.Int64
				attempt := func(ctx context.Context, endpoint string) traceAttemptResult {
					started.Add(1)
					defer exited.Add(1)

					if endpoint == "primary" && test.hasPlannedResult {
						timer := time.NewTimer(10 * time.Second)
						defer timer.Stop()
						select {
						case <-timer.C:
							return traceTestAttemptResult(test.plannedStatus)
						case <-ctx.Done():
							return traceTestAttemptResult(traceAttemptCanceled)
						}
					}

					<-ctx.Done()
					return traceTestAttemptResult(traceAttemptCanceled)
				}
				run := runCloudflareTraceAttempts(probe, test.endpoints, test.hedgeDelay, attempt)

				require.True(t, probe.triggered.Load(), "target cancellation check was not observed before the window closed")
				require.Equal(t, probe.target, probe.observations.Load())
				require.Equal(t, int64(1), started.Load())
				require.Equal(t, started.Load(), exited.Load(), "coordinator returned before every started worker exited")
				require.Equal(t, -1, run.winnerIndex)
				require.False(t, run.timedOut)
				requireTraceStatuses(t, run, test.wantStatuses...)
			})
		})
	}
}

func TestRunCloudflareTraceAttemptsEmptyEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		newCtx   func() (context.Context, context.CancelFunc)
		timedOut bool
	}{
		{
			name: "active-context",
			newCtx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			timedOut: false,
		},
		{
			name: "expired-deadline",
			newCtx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			timedOut: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := test.newCtx()
			defer cancel()
			var attempts atomic.Int64
			run := runCloudflareTraceAttempts(
				ctx,
				nil,
				time.Hour,
				func(context.Context, string) traceAttemptResult {
					attempts.Add(1)
					return traceTestAttemptResult(traceAttemptSucceeded)
				},
			)

			require.EqualValues(t, 0, attempts.Load())
			require.Equal(t, -1, run.winnerIndex)
			require.Equal(t, test.timedOut, run.timedOut)
			require.Empty(t, run.attempts)
		})
	}
}

func TestRunCloudflareTraceAttemptsCallsEachEndpointAtMostOnce(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		endpoints := []string{"primary", "second", "third"}
		controls := newTraceAttemptControls(endpoints)
		done := startCloudflareTraceRun(
			context.Background(), endpoints, 100*time.Millisecond, controlledTraceAttempt(controls),
		)

		receiveTraceStart(t, controls["primary"])
		receiveTraceStart(t, controls["second"])
		controls["primary"].release <- traceTestAttemptResult(traceAttemptFailed)
		controls["second"].release <- traceTestAttemptResult(traceAttemptFailed)
		receiveTraceStart(t, controls["third"])
		controls["third"].release <- traceTestAttemptResult(traceAttemptFailed)
		run := <-done

		require.Equal(t, -1, run.winnerIndex)
		require.False(t, run.timedOut)
		requireTraceStatuses(t, run,
			traceAttemptFailed, traceAttemptFailed, traceAttemptFailed)
		for _, endpoint := range endpoints {
			requireNoTraceStart(t, controls[endpoint])
		}
	})
}
