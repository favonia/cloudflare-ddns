package pp

import (
	"fmt"
	"io"
	"strings"
)

type formatter struct {
	writer io.Writer
	indent int
	level  Level
}

func New(writer io.Writer) PP {
	return &formatter{
		writer: writer,
		indent: 0,
		level:  DefaultLevel,
	}
}

func (f *formatter) SetLevel(lvl Level) PP {
	return &formatter{
		writer: f.writer,
		indent: f.indent,
		level:  lvl,
	}
}

func (f *formatter) IsEnabledFor(lvl Level) bool {
	return lvl >= f.level
}

func (f *formatter) IncIndent() PP {
	return &formatter{
		writer: f.writer,
		indent: f.indent + 1,
		level:  f.level,
	}
}

func (f *formatter) output(lvl Level, emoji Emoji, msg string) {
	if lvl < f.level {
		return
	}

	line := fmt.Sprintf("%s%s %s",
		strings.Repeat(indentPrefix, f.indent),
		string(emoji),
		msg)
	line = strings.TrimSuffix(line, "\n")
	fmt.Fprintln(f.writer, line)
}

func (f *formatter) printf(lvl Level, emoji Emoji, format string, args ...interface{}) {
	f.output(lvl, emoji, fmt.Sprintf(format, args...))
}

func (f *formatter) Infof(emoji Emoji, format string, args ...interface{}) {
	f.printf(Info, emoji, format, args...)
}

func (f *formatter) Noticef(emoji Emoji, format string, args ...interface{}) {
	f.printf(Notice, emoji, format, args...)
}

func (f *formatter) Warningf(emoji Emoji, format string, args ...interface{}) {
	f.printf(Warning, emoji, format, args...)
}

func (f *formatter) Errorf(emoji Emoji, format string, args ...interface{}) {
	f.printf(Error, emoji, format, args...)
}
