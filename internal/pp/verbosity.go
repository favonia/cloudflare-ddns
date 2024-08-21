package pp

// Verbosity is the type of message levels.
type Verbosity int

// Pre-defined verbosity levels.
const (
	Info             Verbosity = iota // useful additional info
	Notice                            // important messages
	Verbose          Verbosity = Info
	Quiet            Verbosity = Notice
	DefaultVerbosity Verbosity = Verbose
)
