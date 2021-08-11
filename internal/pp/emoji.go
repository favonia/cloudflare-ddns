package pp

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

const indentPrefix = "   "
