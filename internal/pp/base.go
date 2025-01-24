// Package pp handles pretty-printing.
package pp

//go:generate mockgen -typed -destination=../mocks/mock_pp.go -package=mocks . PP

// Verbosity is the type of message levels.
type Verbosity int

// Pre-defined verbosity levels. A higher level means "more verbose".
const (
	Notice           Verbosity = iota // useful additional info
	Info                              // important messages
	Quiet            Verbosity = Notice
	Verbose          Verbosity = Info
	DefaultVerbosity Verbosity = Verbose
)

// PP is the abstraction of a pretty printer.
type PP interface {
	// IsShowing checks whether a message of a certain level will be printed.
	IsShowing(v Verbosity) bool

	// Indent returns a new pretty-printer with more indentation.
	Indent() PP

	// BlankLineIfVerbose prints a blank line at the [Verbose] level
	BlankLineIfVerbose()

	// Infof formats and prints a message at the info level.
	Infof(emoji Emoji, format string, args ...any)

	// Noticef formats and prints a message at the notice level.
	Noticef(emoji Emoji, format string, args ...any)

	// Suppress suppresses all future calls to [InfoOncef] and [NoticeOncef]
	// with the same message ID.
	Suppress(id ID)

	// InfoOncef formats and prints an info.
	InfoOncef(id ID, emoji Emoji, format string, args ...any)

	// NoticeOncef formats and prints a notice.
	NoticeOncef(id ID, emoji Emoji, format string, args ...any)
}
