package protocol

type Switch interface {
	Switch(use1001 bool) string
}

type Constant string

func (c Constant) Switch(_ bool) string { return string(c) }

type Switchable struct {
	ValueFor1001 string
	ValueFor1111 string
}

func (s Switchable) Switch(use1001 bool) string {
	if use1001 {
		return s.ValueFor1001
	} else {
		return s.ValueFor1111
	}
}
