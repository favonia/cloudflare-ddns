package cron

import (
	"fmt"
	"time"
)

func PrintPhrase(d time.Duration) string {
	switch {
	case d <= 0:
		return "immediately"
	case d < time.Second:
		return "in less than 1s"
	default:
		return fmt.Sprintf("in about %v", d.Round(time.Second))
	}
}
