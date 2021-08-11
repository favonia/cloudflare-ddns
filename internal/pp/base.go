package pp

type Fmt interface {
	GetLevel() Level
	SetLevel(Level)
	IsEnabledFor(Level) bool
	IncIndent() Fmt
	Printf(Level, Emoji, string, ...interface{})
	Debugf(Emoji, string, ...interface{})
	Infof(Emoji, string, ...interface{})
	Noticef(Emoji, string, ...interface{})
	Warningf(Emoji, string, ...interface{})
	Errorf(Emoji, string, ...interface{})
}
