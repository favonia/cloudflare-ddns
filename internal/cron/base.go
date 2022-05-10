// Package cron handles anything related to time.
package cron

import "time"

// Schedule tells the next time a scheduled event should happen.
type Schedule = interface {
	Next() time.Time
	String() string
}
