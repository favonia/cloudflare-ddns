package cron

import (
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

type Schedule = interface {
	Next() time.Time
	String() string
}

type Cron struct {
	spec     string
	schedule cron.Schedule
}

func New(spec string) (*Cron, bool) {
	sche, err := cron.ParseStandard(spec)
	if err != nil {
		log.Printf(`😡 Could not parse %s as a Cron expresion: %v`, spec, err)
		return nil, false
	}

	return &Cron{
		spec:     spec,
		schedule: sche,
	}, true
}

func MustNew(spec string) *Cron {
	cron, ok := New(spec)
	if !ok {
		panic("schedule.MustNew: assertion failed")
	}
	return cron
}

func (s *Cron) Next() time.Time {
	return s.schedule.Next(time.Now())
}

func (s *Cron) String() string {
	return s.spec
}
