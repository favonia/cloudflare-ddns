package cron

import (
	"time"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

const (
	intervalUnit     time.Duration = time.Second
	intervalLargeGap time.Duration = time.Second * 5
	intervalHugeGap  time.Duration = time.Minute * 10
)

func DescribeIntuitively(now, target time.Time) string {
	now = now.In(time.Local)
	target = target.In(time.Local)

	switch {
	case now.Year() != target.Year():
		return target.In(time.Local).Format("02 Jan 15:04 2006")
	case now.YearDay() != target.YearDay():
		return target.In(time.Local).Format("02 Jan 15:04")
	default:
		return target.In(time.Local).Format("15:04")
	}
}

func PrintCountdown(ppfmt pp.PP, activity string, now, target time.Time) {
	interval := target.Sub(now)

	switch {
	case interval < -intervalLargeGap:
		ppfmt.Infof(pp.EmojiNow, "%s now (running behind by %v) . . .", activity, -interval.Round(intervalUnit))
	case interval < intervalUnit:
		ppfmt.Infof(pp.EmojiNow, "%s now . . .", activity)
	case interval < intervalLargeGap:
		ppfmt.Infof(pp.EmojiAlarm, "%s in less than %v . . .", activity, intervalLargeGap)
	case interval < intervalHugeGap:
		ppfmt.Infof(pp.EmojiAlarm, "%s in about %v . . .", activity, interval.Round(intervalUnit))
	default:
		ppfmt.Infof(pp.EmojiAlarm, "%s in about %v (%v) . . .",
			activity,
			interval.Round(intervalUnit),
			DescribeIntuitively(now, target),
		)
	}
}
