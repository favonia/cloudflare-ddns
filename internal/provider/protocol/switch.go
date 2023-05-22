package protocol

type Switch interface {
	Switch(use1001 bool) string
}

type Constant string

func (c Constant) Switch(_ bool) string { return string(c) }

type Switchable struct {
	Use1111 string
	Use1001 string
}

func (s Switchable) Switch(use1001 bool) string {
	if use1001 {
		return s.Use1001
	} else {
		return s.Use1111
	}
}
