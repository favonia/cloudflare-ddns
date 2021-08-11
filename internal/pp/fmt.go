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

func New(writer io.Writer) Fmt {
	return &formatter{
		writer: writer,
		indent: 0,
		level:  DefaultLevel,
	}
}

func (f *formatter) GetLevel() Level {
	return f.level
}

func (f *formatter) SetLevel(lvl Level) {
	f.level = lvl
}

func (f *formatter) IsEnabledFor(lvl Level) bool {
	return lvl >= f.level
}

func (f *formatter) IncIndent() Fmt {
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

func (f *formatter) Printf(lvl Level, emoji Emoji, format string, args ...interface{}) {
	f.output(lvl, emoji, fmt.Sprintf(format, args...))
}

func (f *formatter) Debugf(emoji Emoji, format string, args ...interface{}) {
	f.Printf(Debug, emoji, format, args...)
}

func (f *formatter) Infof(emoji Emoji, format string, args ...interface{}) {
	f.Printf(Info, emoji, format, args...)
}

func (f *formatter) Noticef(emoji Emoji, format string, args ...interface{}) {
	f.Printf(Notice, emoji, format, args...)
}

func (f *formatter) Warningf(emoji Emoji, format string, args ...interface{}) {
	f.Printf(Warning, emoji, format, args...)
}

func (f *formatter) Errorf(emoji Emoji, format string, args ...interface{}) {
	f.Printf(Error, emoji, format, args...)
}
