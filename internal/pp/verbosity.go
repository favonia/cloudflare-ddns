package pp

// Level is the type of message levels.
type VerbosityLevel int

const (
	Debug            VerbosityLevel = iota // debugging info, currently not used
	Info                                   // additional information that is not an action, a warning, or an error
	Notice                                 // an action (e.g., changing the IP) has happened and it is not an error
	Warning                                // non-fatal errors where the updater should continue updating IP addresses
	Error                                  // fatal errors where the updater should stop
	DefaultVerbosity VerbosityLevel = Info
	Verbose          VerbosityLevel = Info
	Quiet            VerbosityLevel = Notice
)
