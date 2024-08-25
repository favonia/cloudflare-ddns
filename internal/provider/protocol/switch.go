package protocol

// Method denotes the choice between the primary and alternative methods.
type Method int

// The tags for the primary and alternative methods.
const (
	MethodUnspecified Method = iota
	MethodPrimary
	MethodAlternative
)

// Switch represents a string depending on the method.
type Switch interface {
	Switch(method Method) string
	HasAlternative() bool
}

// Constant is a [Switch] that ignores whether 1.1.1.1 or 1.0.0.1 is used.
type Constant string

// Switch always returns the same string.
func (c Constant) Switch(Method) string { return string(c) }

// HasAlternative always returns false.
func (Constant) HasAlternative() bool { return false }

// Switchable is a [Switch] that returns one of the given strings.
type Switchable struct {
	Primary     string
	Alternative string
}

// Switch returns one of the given strings depending on the boolean value.
func (s Switchable) Switch(method Method) string {
	switch method {
	default:
		return s.Primary
	case MethodAlternative:
		return s.Alternative
	}
}

// HasAlternative always returns true.
func (Switchable) HasAlternative() bool { return true }
