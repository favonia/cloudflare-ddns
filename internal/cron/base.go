// Package cron handles anything related to time.
package cron

import "time"

// Schedule tells the next time a scheduled event should happen.
type Schedule = interface {
	Next() time.Time
	Describe() string
}

// Next gets the next scheduled time. It returns the zero value for nil.
func Next(s Schedule) time.Time {
	if s == nil {
		return time.Time{}
	}

	return s.Next()
}

// DescribeSchedule gives back the original cron string.
func DescribeSchedule(s Schedule) string {
	if s == nil {
		return "@once"
	}

	return s.Describe()
}
