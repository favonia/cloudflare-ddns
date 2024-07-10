package cron_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestDescribeIntuitively(t *testing.T) {
	t.Parallel()

	now := time.Now().In(time.Local)
	nextYear := now.AddDate(1, 0, 0)
	diffDay := now.AddDate(0, 0, 1)
	if diffDay.Year() != now.Year() {
		diffDay = now.AddDate(0, 0, -1)
	}

	for name, tc := range map[string]struct {
		time   time.Time
		output string
	}{
		"now": {
			now,
			now.Format("15:04"),
		},
		"1day": {
			diffDay,
			diffDay.Format("02 Jan 15:04"),
		},
		"1year": {
			nextYear,
			nextYear.Format("02 Jan 15:04 2006"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.output, cron.DescribeIntuitively(now, tc.time))
		})
	}
}

func TestPrintCountdown(t *testing.T) {
	t.Parallel()

	activity := "Secretly dancing"

	for _, tc := range [...]struct {
		interval []time.Duration
		output   func(string) string
	}{
		{
			[]time.Duration{-20 * time.Second},
			func(_ string) string {
				return string(pp.EmojiNow) + " Secretly dancing now (running behind by 20s) . . .\n"
			},
		},
		{
			[]time.Duration{-10 * time.Second},
			func(_ string) string {
				return string(pp.EmojiNow) + " Secretly dancing now (running behind by 10s) . . .\n"
			},
		},
		{
			[]time.Duration{-time.Second, -time.Nanosecond, 0, time.Nanosecond},
			func(_ string) string {
				return string(pp.EmojiNow) + " Secretly dancing now . . .\n"
			},
		},
		{
			[]time.Duration{2 * time.Second},
			func(_ string) string {
				return string(pp.EmojiAlarm) + " Secretly dancing in less than 5s . . .\n"
			},
		},
		{
			[]time.Duration{10 * time.Second},
			func(_ string) string {
				return string(pp.EmojiAlarm) + " Secretly dancing in about 10s . . .\n"
			},
		},
		{
			[]time.Duration{20 * time.Second},
			func(_ string) string {
				return string(pp.EmojiAlarm) + " Secretly dancing in about 20s . . .\n"
			},
		},
		{
			[]time.Duration{20 * time.Minute},
			func(t string) string {
				return string(pp.EmojiAlarm) + " Secretly dancing in about 20m0s (" + t + ") . . .\n"
			},
		},
	} {
		for _, interval := range tc.interval {
			t.Run(interval.String(), func(t *testing.T) {
				t.Parallel()
				var buf strings.Builder
				pp := pp.New(&buf)

				now := time.Now()
				target := now.Add(interval)
				cron.PrintCountdown(pp, activity, now, target)
				require.Equal(t, tc.output(cron.DescribeIntuitively(now, target)), buf.String())
			})
		}
	}
}
