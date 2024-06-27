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

func PrintCountdown(ppfmt pp.PP, activity string, target time.Time) {
	interval := time.Until(target)

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
			target.In(time.Local).Format(time.Kitchen),
		)
	}
}
