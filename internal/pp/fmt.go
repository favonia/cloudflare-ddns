package pp

import (
	"fmt"
	"io"
	"strings"
)

type formatter struct {
	writer       io.Writer
	emoji        bool
	indent       int
	messageShown map[ID]bool
	verbosity    Verbosity
}

// New creates a new pretty printer.
func New(writer io.Writer, emoji bool, verbosity Verbosity) PP {
	return formatter{
		writer:       writer,
		emoji:        emoji,
		indent:       0,
		messageShown: map[ID]bool{},
		verbosity:    verbosity,
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

// BlankLineIfVerbose prints a blank line.
func (f formatter) BlankLineIfVerbose() {
	if f.IsShowing(Verbose) {
		fmt.Fprintln(f.writer)
	}
}

// Infof formats and sends a message at the level [Info].
func (f formatter) Infof(emoji Emoji, format string, args ...any) {
	f.printf(Info, emoji, format, args...)
}

// Noticef formats and sends a message at the level [Notice].
func (f formatter) Noticef(emoji Emoji, format string, args ...any) {
	f.printf(Notice, emoji, format, args...)
}

// Suppress sets the hint in the internal map to be "shown".
func (f formatter) Suppress(id ID) {
	f.messageShown[id] = true
}

// InfoOncef calls [Infof] for if the message ID is new, and ignore it otherwise.
func (f formatter) InfoOncef(id ID, emoji Emoji, format string, args ...any) {
	if !f.messageShown[id] {
		f.Infof(emoji, format, args...)
		f.messageShown[id] = true
	}
}

// NoticeOncef calls [Noticf] for if the message ID is new, and ignore it otherwise.
func (f formatter) NoticeOncef(id ID, emoji Emoji, format string, args ...any) {
	if !f.messageShown[id] {
		f.Noticef(emoji, format, args...)
		f.messageShown[id] = true
	}
}

// printf composes the message body and forwards it to [output].
func (f formatter) printf(v Verbosity, emoji Emoji, format string, args ...any) {
	f.output(v, emoji, fmt.Sprintf(format, args...))
}

// output prints the message string.
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
