// Package pp handles pretty-printing.
package pp

//go:generate mockgen -typed -destination=../mocks/mock_pp.go -package=mocks . PP

// PP is the abstraction of a pretty printer.
type PP interface {
	// SetEmoji sets whether emojis should be used.
	SetEmoji(emoji bool) PP

	// SetVerbosity sets the level under which messages will be hidden.
	SetVerbosity(v Verbosity) PP

	// IsEnabledFor checks whether a message of a certain level will be displayed.
	IsEnabledFor(v Verbosity) bool

	// IncIndent returns a new pretty-printer with more indentation.
	IncIndent() PP

	// Infof formats and prints a message at the info level.
	Infof(emoji Emoji, format string, args ...any)

	// Noticef formats and prints a message at the notice level.
	Noticef(emoji Emoji, format string, args ...any)

	// Warningf formats and prints a message at the warning level.
	Warningf(emoji Emoji, format string, args ...any)

	// Errorf formats and prints a message at the error level.
	Errorf(emoji Emoji, format string, args ...any)
}
