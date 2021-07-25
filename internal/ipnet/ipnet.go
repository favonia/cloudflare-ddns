package ipnet

import (
	"fmt"
)

type Type int

const (
	IP4 Type = 4
	IP6 Type = 6
)

func (t Type) String() string {
	return fmt.Sprintf("IPv%d", t)
}

func (t Type) RecordType() string {
	switch t {
	case IP4:
		return "A"
	case IP6:
		return "AAAA"
	default:
		return ""
	}
}

func (t Type) Int() int {
	return int(t)
}
