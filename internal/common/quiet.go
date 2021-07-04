package common

type Quiet bool

const (
	QUIET   = Quiet(true)
	VERBOSE = Quiet(false)
)
