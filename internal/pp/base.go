package pp

//go:generate mockgen -destination=../mocks/mock_pp.go -package=mocks . PP

type PP interface {
	SetLevel(Level) PP
	IsEnabledFor(Level) bool
	IncIndent() PP
	Infof(Emoji, string, ...any)
	Noticef(Emoji, string, ...any)
	Warningf(Emoji, string, ...any)
	Errorf(Emoji, string, ...any)
}
