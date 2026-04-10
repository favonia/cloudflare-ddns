package cron_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/cron"
)

func TestJitterDuration(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		interval time.Duration
		wantMin  time.Duration
		wantMax  time.Duration
	}{
		"5min": {
			interval: 5 * time.Minute,
			wantMin:  0,
			wantMax:  60 * time.Second, // interval/5 = 60s, range is [0, 60s)
		},
		"1hour": {
			interval: time.Hour,
			wantMin:  0,
			wantMax:  12 * time.Minute, // interval/5 = 12m, range is [0, 12m)
		},
		"sub5ns": {
			interval: 4 * time.Nanosecond,
			wantMin:  0,
			wantMax:  0, // divisor = 0, must return 0
		},
		"zero": {
			interval: 0,
			wantMin:  0,
			wantMax:  0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Run several iterations to exercise the random range.
			const iterations = 100
			for range iterations {
				got := cron.JitterDuration(tc.interval)
				require.GreaterOrEqual(t, got, tc.wantMin,
					"JitterDuration(%v) = %v, want >= %v", tc.interval, got, tc.wantMin)
				require.LessOrEqual(t, got, tc.wantMax,
					"JitterDuration(%v) = %v, want <= %v", tc.interval, got, tc.wantMax)
			}
		})
	}
}
