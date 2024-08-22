// Package pp handles pretty-printing.
package pp

//go:generate mockgen -typed -destination=../mocks/mock_pp.go -package=mocks . PP

// PP is the abstraction of a pretty printer.
type PP interface {
	// Verbosity returns the verbosity level.
	Verbosity() Verbosity

	// Indent returns a new pretty-printer with more indentation.
	Indent() PP

	// Infof formats and prints a message at the info level.
	Infof(emoji Emoji, format string, args ...any)

	// Noticef formats and prints a message at the notice level.
	Noticef(emoji Emoji, format string, args ...any)

	// SuppressHint suppresses all future calls to [Hintf] with the same hint ID.
	SuppressHint(hint Hint)

	// Hintf formats and prints a hint.
	Hintf(hint Hint, format string, args ...any)
}
