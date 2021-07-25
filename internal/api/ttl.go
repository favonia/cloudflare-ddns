package api

import (
	"strconv"
)

type TTL int

func (t TTL) String() string {
	if t == 1 {
		return "1 (automatic)"
	}
	return strconv.Itoa(int(t))
}

func (t TTL) Int() int {
	return int(t)
}
