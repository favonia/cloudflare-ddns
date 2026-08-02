package protocol

import (
	"context"
	"errors"
	"time"
)

// This 250 ms cap is an uncalibrated heuristic, not a protocol value or a
// measurement-derived optimum. T/20 preserves its ratio to the default 5 s timeout.
const maxCloudflareTraceHedgeDelay = 250 * time.Millisecond

type traceRunResult struct {
	winnerIndex int
	attempts    []traceAttemptResult
	timedOut    bool
}

type traceAttemptFunc func(context.Context, string) traceAttemptResult

type indexedTraceAttemptResult struct {
	index  int
	result traceAttemptResult
}

func cloudflareTraceHedgeDelay(ctx context.Context, now time.Time) time.Duration {
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		return maxCloudflareTraceHedgeDelay
	}
	return min(deadline.Sub(now)/20, maxCloudflareTraceHedgeDelay)
}

func runCloudflareTraceAttempts(
	ctx context.Context,
	endpoints []string,
	hedgeDelay time.Duration,
	attempt traceAttemptFunc,
) traceRunResult {
	run := traceRunResult{
		winnerIndex: -1,
		attempts:    make([]traceAttemptResult, len(endpoints)),
		timedOut:    false,
	}
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan indexedTraceAttemptResult, len(endpoints))
	started := 0
	completed := 0
	next := 0

	parentCanceled := func() bool {
		return context.Cause(ctx) != nil || ctx.Err() != nil
	}
	parentTimedOut := func() bool {
		return errors.Is(context.Cause(ctx), context.DeadlineExceeded) ||
			errors.Is(ctx.Err(), context.DeadlineExceeded)
	}

	var timer *time.Timer
	var timerC <-chan time.Time
	stopTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer = nil
		timerC = nil
	}

	launchNext := func() bool {
		if next >= len(endpoints) {
			return true
		}
		if parentCanceled() {
			return false
		}

		index := next
		endpoint := endpoints[index]
		next++
		started++
		go func() {
			results <- indexedTraceAttemptResult{
				index:  index,
				result: attempt(childCtx, endpoint),
			}
		}()

		stopTimer()
		if hedgeDelay > 0 && next < len(endpoints) {
			timer = time.NewTimer(hedgeDelay)
			timerC = timer.C
		}
		return true
	}

	drain := func() {
		for completed < started {
			attemptResult := <-results
			run.attempts[attemptResult.index] = attemptResult.result
			completed++
		}
	}
	finishCanceled := func() traceRunResult {
		stopTimer()
		cancel()
		drain()
		run.timedOut = parentTimedOut()
		return run
	}
	finishWinner := func(winnerIndex int) traceRunResult {
		run.winnerIndex = winnerIndex
		stopTimer()
		cancel()
		drain()
		return run
	}

	if len(endpoints) == 0 {
		if parentCanceled() {
			run.timedOut = parentTimedOut()
		}
		return run
	}
	if !launchNext() {
		return finishCanceled()
	}
	if hedgeDelay <= 0 {
		for next < len(endpoints) {
			if !launchNext() {
				return finishCanceled()
			}
		}
	}

	for {
		if parentCanceled() {
			return finishCanceled()
		}

		select {
		case <-ctx.Done():
			return finishCanceled()

		case attemptResult := <-results:
			run.attempts[attemptResult.index] = attemptResult.result
			completed++
			if parentCanceled() {
				return finishCanceled()
			}

			switch attemptResult.result.status {
			case traceAttemptSucceeded:
				return finishWinner(attemptResult.index)
			case traceAttemptFailed:
				if !launchNext() {
					return finishCanceled()
				}
			case traceAttemptUnstarted, traceAttemptCanceled:
			}

			if completed == started && next >= len(endpoints) {
				if parentCanceled() {
					return finishCanceled()
				}
				stopTimer()
				return run
			}

		case <-timerC:
			if !launchNext() {
				return finishCanceled()
			}
		}
	}
}
