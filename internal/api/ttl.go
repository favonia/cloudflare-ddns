package api

import (
	"strconv"
)

type TTL int

func (t TTL) Int() int {
	return int(t)
}

func (t TTL) String() string {
	return strconv.Itoa(t.Int())
}

func (t TTL) Describe() string {
	if t == 1 {
		return "1 (automatic)"
	}
	return strconv.Itoa(t.Int())
}
