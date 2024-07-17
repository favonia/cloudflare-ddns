package pp

// Verbosity is the type of message levels.
type Verbosity int

// Pre-defined verbosity levels.
const (
	Debug            Verbosity = iota // debugging info (currently not used)
	Info                              // additional information that is not an action, a warning, or an error
	Notice                            // an action (e.g., changing the IP) that is not an error
	Warning                           // non-fatal errors where we should continue updating IP addresses
	Error                             // fatal errors where the updater should stop
	Verbose          Verbosity = Info
	Quiet            Verbosity = Notice
	DefaultVerbosity Verbosity = Verbose
)
