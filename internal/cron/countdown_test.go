package cron_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestPrintCountdown(t *testing.T) {
	t.Parallel()

	activity := "Secretly dancing"

	for _, tc := range [...]struct {
		interval []time.Duration
		output   string
	}{
		{
			[]time.Duration{-20 * time.Second},
			string(pp.EmojiNow) + " Secretly dancing now (running behind by 20s) . . .\n",
		},
		{
			[]time.Duration{-10 * time.Second},
			string(pp.EmojiNow) + " Secretly dancing now (running behind by 10s) . . .\n",
		},
		{
			[]time.Duration{-time.Second, -time.Nanosecond, 0, time.Nanosecond},
			string(pp.EmojiNow) + " Secretly dancing now . . .\n",
		},
		{
			[]time.Duration{time.Second},
			string(pp.EmojiAlarm) + " Secretly dancing in less than 5s . . .\n",
		},
		{
			[]time.Duration{10 * time.Second},
			string(pp.EmojiAlarm) + " Secretly dancing in about 10s . . .\n",
		},
		{
			[]time.Duration{20 * time.Second},
			string(pp.EmojiAlarm) + " Secretly dancing in about 20s . . .\n",
		},
	} {
		for _, interval := range tc.interval {
			t.Run(interval.String(), func(t *testing.T) {
				t.Parallel()
				var buf strings.Builder
				pp := pp.New(&buf)

				cron.PrintCountdown(pp, activity, interval)

				require.Equal(t, tc.output, buf.String())
			})
		}
	}
}
