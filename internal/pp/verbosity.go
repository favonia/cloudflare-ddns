package pp

// Level is the type of message levels.
type Verbosity int

const (
	Debug            Verbosity = iota // debugging info, currently not used
	Info                              // additional information that is not an action, a warning, or an error
	Notice                            // an action (e.g., changing the IP) has happened and it is not an error
	Warning                           // non-fatal errors where the updater should continue updating IP addresses
	Error                             // fatal errors where the updater should stop
	DefaultVerbosity Verbosity = Info
	Verbose          Verbosity = Info
	Quiet            Verbosity = Notice
)
