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
	verbosity Verbosity
}

// New creates a new pretty printer.
func New(writer io.Writer) PP {
	return formatter{
		writer:    writer,
		emoji:     true,
		indent:    0,
		verbosity: DefaultVerbosity,
	}
}

// SetEmoji sets whether emojis should be printed.
func (f formatter) SetEmoji(emoji bool) PP {
	f.emoji = emoji
	return f
}

// SetVerbosity sets messages of what verbosity levels should be printed.
func (f formatter) SetVerbosity(v Verbosity) PP {
	f.verbosity = v
	return f
}

// IsEnabledFor checks whether a message of verbosity level v will be printed.
func (f formatter) IsEnabledFor(v Verbosity) bool {
	return v >= f.verbosity
}

// IncIndent returns a new printer that indents the messages more than the input printer.
func (f formatter) IncIndent() PP {
	f.indent++
	return f
}

func (f formatter) output(v Verbosity, emoji Emoji, msg string) {
	if v < f.verbosity {
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

// Warningf formats and sends a message at the level [Warning].
func (f formatter) Warningf(emoji Emoji, format string, args ...any) {
	f.printf(Warning, emoji, format, args...)
}

// Errorf formats and sends a message at the level [Error].
func (f formatter) Errorf(emoji Emoji, format string, args ...any) {
	f.printf(Error, emoji, format, args...)
}
