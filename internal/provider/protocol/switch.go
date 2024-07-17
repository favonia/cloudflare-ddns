package protocol

// Switch represents a string that depends on whether 1.1.1.1 or 1.0.0.1 should be used.
type Switch interface {
	Switch(use1001 bool) string
}

// Constant is a [Switch] that ignores whether 1.1.1.1 or 1.0.0.1 is used.
type Constant string

// Switch always returns the same string.
func (c Constant) Switch(_ bool) string { return string(c) }

// Switchable is a [Switch] that returns one of the given strings.
type Switchable struct {
	ValueFor1001 string
	ValueFor1111 string
}

// Switch returns one of the given strings depending on whether 1.1.1.1 or 1.0.0.1 should be used.
func (s Switchable) Switch(use1001 bool) string {
	if use1001 {
		return s.ValueFor1001
	} else {
		return s.ValueFor1111
	}
}
