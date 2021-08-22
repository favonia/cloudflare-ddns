package pp

//go:generate mockgen -destination=../mocks/mock_pp.go -package=mocks . PP

type PP interface {
	GetLevel() Level
	SetLevel(Level)
	IsEnabledFor(Level) bool
	IncIndent() PP
	Infof(Emoji, string, ...interface{})
	Noticef(Emoji, string, ...interface{})
	Warningf(Emoji, string, ...interface{})
	Errorf(Emoji, string, ...interface{})
}
