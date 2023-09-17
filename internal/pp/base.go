// Package pp handles pretty-printing.
package pp

//go:generate mockgen -typed -destination=../mocks/mock_pp.go -package=mocks . PP

// PP is the abstraction of a pretty printer.
type PP interface {
	// SetEmoji sets whether emojis should be used.
	SetEmoji(bool) PP

	// SetLevel sets the level under which messages will be hidden.
	SetLevel(Level) PP

	// IsEnabledFor checks whether a message of a certain level will be displayed.
	IsEnabledFor(Level) bool

	// IncIndent returns a new pretty-printer with more indentation.
	IncIndent() PP

	// Infof formats and prints a message at the info level.
	Infof(Emoji, string, ...any)

	// Noticef formats and prints a message at the notice level.
	Noticef(Emoji, string, ...any)

	// Warningf formats and prints a message at the warning level.
	Warningf(Emoji, string, ...any)

	// Errorf formats and prints a message at the error level.
	Errorf(Emoji, string, ...any)
}
