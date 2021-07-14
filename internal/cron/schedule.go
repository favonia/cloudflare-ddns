package cron

import (
	"log"
	"time"

	c "github.com/robfig/cron/v3"
)

type Schedule = interface {
	Next() time.Time
	String() string
}

type Cron struct {
	spec     string
	schedule c.Schedule
}

func New(spec string) (*Cron, error) {
	sche, err := c.ParseStandard(spec)
	if err != nil {
		return nil, err
	}

	return &Cron{
		spec:     spec,
		schedule: sche,
	}, nil
}

func MustNew(spec string) *Cron {
	cron, err := New(spec)
	if err != nil {
		log.Fatal(err)
	}
	return cron
}

func (s *Cron) Next() time.Time {
	return s.schedule.Next(time.Now())
}

func (s *Cron) String() string {
	return s.spec
}
