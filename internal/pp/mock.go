package pp

import "fmt"

type Record struct {
	Indent  int
	Level   Level
	Emoji   Emoji
	Message string
}

func NewRecord(indent int, level Level, emoji Emoji, message string) Record {
	return Record{indent, level, emoji, message}
}

type Mock struct {
	Records []Record
	indent  int
	Level   Level
}

func NewMock() *Mock {
	return &Mock{
		Records: nil,
		indent:  0,
		Level:   DefaultLevel,
	}
}

func (f *Mock) Clear() {
	f.Records = nil
}

func (f *Mock) GetLevel() Level {
	return f.Level
}

func (f *Mock) SetLevel(lvl Level) {
	f.Level = lvl
}

func (f *Mock) IsEnabledFor(lvl Level) bool {
	return lvl >= f.Level
}

func (f *Mock) IncIndent() Fmt {
	return &Mock{
		Records: f.Records,
		indent:  f.indent + 1,
		Level:   f.Level,
	}
}

func (f *Mock) Printf(lvl Level, emoji Emoji, format string, args ...interface{}) {
	if lvl < f.Level {
		return
	}

	f.Records = append(f.Records, Record{
		Indent:  f.indent,
		Level:   lvl,
		Emoji:   emoji,
		Message: fmt.Sprintf(format, args...),
	})
}

func (f *Mock) Debugf(emoji Emoji, format string, args ...interface{}) {
	f.Printf(Debug, emoji, format, args...)
}

func (f *Mock) Infof(emoji Emoji, format string, args ...interface{}) {
	f.Printf(Info, emoji, format, args...)
}

func (f *Mock) Noticef(emoji Emoji, format string, args ...interface{}) {
	f.Printf(Notice, emoji, format, args...)
}

func (f *Mock) Warningf(emoji Emoji, format string, args ...interface{}) {
	f.Printf(Warning, emoji, format, args...)
}

func (f *Mock) Errorf(emoji Emoji, format string, args ...interface{}) {
	f.Printf(Error, emoji, format, args...)
}
