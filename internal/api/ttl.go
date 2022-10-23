package api

import "strconv"

type TTL int

const TTLAuto TTL = 1

func (t TTL) Int() int {
	return int(t)
}

func (t TTL) String() string {
	return strconv.Itoa(t.Int())
}

func (t TTL) Describe() string {
	if t == TTLAuto {
		return "1 (auto)"
	}
	return strconv.Itoa(t.Int())
}
