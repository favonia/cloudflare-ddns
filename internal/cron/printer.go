package cron

import (
	"fmt"
	"time"
)

func PPDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "no time "
	case d < time.Second:
		return "less than 1s"
	default:
		return fmt.Sprintf("about %v", d.Round(time.Second))
	}
}
