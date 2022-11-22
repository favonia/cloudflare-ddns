package pp

import (
	"fmt"
	"io"
	"strings"
)

type formatter struct {
	writer io.Writer
	emoji  bool
	indent int
	level  Level
}

func New(writer io.Writer) PP {
	return &formatter{
		writer: writer,
		emoji:  true,
		indent: 0,
		level:  DefaultLevel,
	}
}

func (f *formatter) SetEmoji(emoji bool) PP {
	fmt := *f
	fmt.emoji = emoji
	return &fmt
}

func (f *formatter) SetLevel(lvl Level) PP {
	fmt := *f
	fmt.level = lvl
	return &fmt
}

func (f *formatter) IsEnabledFor(lvl Level) bool {
	return lvl >= f.level
}

func (f *formatter) IncIndent() PP {
	fmt := *f
	fmt.indent++
	return &fmt
}

func (f *formatter) output(lvl Level, emoji Emoji, msg string) {
	if lvl < f.level {
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

func (f *formatter) printf(lvl Level, emoji Emoji, format string, args ...any) {
	f.output(lvl, emoji, fmt.Sprintf(format, args...))
}

func (f *formatter) Infof(emoji Emoji, format string, args ...any) {
	f.printf(Info, emoji, format, args...)
}

func (f *formatter) Noticef(emoji Emoji, format string, args ...any) {
	f.printf(Notice, emoji, format, args...)
}

func (f *formatter) Warningf(emoji Emoji, format string, args ...any) {
	f.printf(Warning, emoji, format, args...)
}

func (f *formatter) Errorf(emoji Emoji, format string, args ...any) {
	f.printf(Error, emoji, format, args...)
}
