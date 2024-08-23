package pp

import (
	"fmt"
	"io"
	"strings"
)

type formatter struct {
	writer    io.Writer
	emoji     bool
	indent    int
	hintShown map[Hint]bool
	verbosity Verbosity
}

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

// New creates a new pretty printer.
func New(writer io.Writer, emoji bool, verbosity Verbosity) PP {
	return formatter{
		writer:    writer,
		emoji:     emoji,
		indent:    0,
		hintShown: map[Hint]bool{},
		verbosity: verbosity,
	}
}

// NewDefault creates a new pretty printer with default settings.
func NewDefault(writer io.Writer) PP {
	return New(writer, true, DefaultVerbosity)
}

// IsShowing compares the internal verbosity level against the given level.
func (f formatter) IsShowing(v Verbosity) bool {
	return f.verbosity >= v
}

// Indent returns a new printer that indents the messages more than the input printer.
func (f formatter) Indent() PP {
	f.indent++
	return f
}

func (f formatter) output(v Verbosity, emoji Emoji, msg string) {
	if !f.IsShowing(v) {
		return
	}

	var line string
	if f.emoji {
		line = fmt.Sprintf("%s%s %s",
			strings.Repeat(indentPrefix, f.indent),
			string(emoji),
			msg)
	} else {
		line = fmt.Sprintf("%s%s",
			strings.Repeat(indentPrefix, f.indent),
			msg)
	}
	line = strings.TrimSuffix(line, "\n")
	fmt.Fprintln(f.writer, line)
}

func (f formatter) printf(v Verbosity, emoji Emoji, format string, args ...any) {
	f.output(v, emoji, fmt.Sprintf(format, args...))
}

// Infof formats and sends a message at the level [Info].
func (f formatter) Infof(emoji Emoji, format string, args ...any) {
	f.printf(Info, emoji, format, args...)
}

// Noticef formats and sends a message at the level [Notice].
func (f formatter) Noticef(emoji Emoji, format string, args ...any) {
	f.printf(Notice, emoji, format, args...)
}

// SuppressHint sets the hint in the internal map to be "shown".
func (f formatter) SuppressHint(hint Hint) {
	f.hintShown[hint] = true
}

// Hintf calls [Infof] with the emoji [EmojiHint].
func (f formatter) Hintf(hint Hint, format string, args ...any) {
	if !f.hintShown[hint] {
		f.Infof(EmojiHint, format, args...)
		f.hintShown[hint] = true
	}
}
