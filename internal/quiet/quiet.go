package quiet

type Quiet bool

const (
	QUIET   = Quiet(true)
	VERBOSE = Quiet(false)
	Default = VERBOSE
)
