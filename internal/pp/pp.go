package pp

import (
	"fmt"
	"strings"
)

type Indent int

const NoIndent Indent = 0

func (i Indent) Succ() Indent {
	return i + 1
}

type Emoji string

const (
	EmojiStar   Emoji = "🌟" // stars attached to the tool name
	EmojiBullet Emoji = "🔸" // generic bullet points

	EmojiEnvVars     Emoji = "📖" // reading configuration
	EmojiConfig      Emoji = "🔧" // showing configuration
	EmojiInternet    Emoji = "🌐" // network address detection
	EmojiPriviledges Emoji = "🥷" // /privileges
	EmojiMute        Emoji = "🔇" // quiet mode

	EmojiAddRecord    Emoji = "🐣" // adding new DNS records
	EmojiDelRecord    Emoji = "💀" // deleting DNS records
	EmojiUpdateRecord Emoji = "📡" // updating DNS records

	EmojiSignal      Emoji = "🚨" // catching signals
	EmojiAlreadyDone Emoji = "🤷" // DNS records were already up to date
	EmojiNow         Emoji = "🏃" // an event that is happening now or immediately
	EmojiAlarm       Emoji = "⏰" // an event that is scheduled to happen, but not immediately
	EmojiBye         Emoji = "👋" // bye!

	EmojiUserError   Emoji = "😡" // configuration mistakes made by users
	EmojiUserWarning Emoji = "😦" // warnings about possible configuration mistakes
	EmojiError       Emoji = "😞" // errors that are not (directly) caused by user errors
	EmojiImpossible  Emoji = "🤯" // the impossible happened
	EmojiGood        Emoji = "👍" // everything looks good
)

func (e Emoji) String() string {
	return string(e)
}

const IndentPrefix = "   "

func prefix(indent Indent, emoji Emoji, msg string) string {
	return fmt.Sprintf("%s%s %s", strings.Repeat(IndentPrefix, int(indent)), emoji, msg)
}

func printString(indent Indent, emoji Emoji, msg string) {
	buf := []byte(prefix(indent, emoji, msg))
	if len(buf) == 0 || buf[len(buf)-1] != '\n' {
		buf = append(buf, '\n')
	}
	fmt.Printf("%s", buf) //nolint:forbidigo
}

func Print(indent Indent, emoji Emoji, args ...interface{}) {
	if false { // enable go vet printf checking
		_ = fmt.Sprint(args...)
	}

	printString(indent, emoji, fmt.Sprint(args...))
}

func Printf(indent Indent, emoji Emoji, format string, args ...interface{}) {
	if false { // enable go vet printf checking
		_ = fmt.Sprintf(format, args...)
	}

	printString(indent, emoji, fmt.Sprintf(format, args...))
}

func TopPrint(emoji Emoji, args ...interface{}) {
	if false { // enable go vet printf checking
		_ = fmt.Sprint(args...)
	}

	Print(NoIndent, emoji, args...)
}

func TopPrintf(emoji Emoji, format string, args ...interface{}) {
	if false { // enable go vet printf checking
		_ = fmt.Sprintf(format, args...)
	}

	Printf(NoIndent, emoji, format, args...)
}
