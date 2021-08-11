package pp

type Level int

const (
	Debug        Level = iota // debugging info, currently not used
	Info                      // information not about actual actions
	Notice                    // information about actual actions, but not an error
	Warning                   // non-fatal errors where the program should continue updating IP addresses
	Error                     // fatal errors where the program should stop
	DefaultLevel = Info
	Verbose      = Info
	Quiet        = Notice
)
