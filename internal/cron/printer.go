package cron

import (
	"fmt"
	"time"
)

func PPDuration(d time.Duration) string {
	if d < time.Second {
		return "less than 1s"
	}

	return fmt.Sprintf("about %v", d.Round(time.Second))
}
