package pp

// Emoji is the type of emoji strings.
type Emoji string

const (
	EmojiStar   Emoji = "🌟" // stars attached to the tool name
	EmojiBullet Emoji = "🔸" // generic bullet points

	EmojiEnvVars      Emoji = "📖" // reading configuration
	EmojiConfig       Emoji = "🔧" // showing configuration
	EmojiInternet     Emoji = "🌐" // network address detection
	EmojiPrivileges   Emoji = "🥷" // /privileges
	EmojiMute         Emoji = "🔇" // quiet mode
	EmojiDisabled     Emoji = "🚫" // feature is disabled
	EmojiExperimental Emoji = "🧪" // experimental features

	EmojiCreateRecord Emoji = "🐣" // adding new DNS records
	EmojiDeleteRecord Emoji = "💀" // deleting DNS records
	EmojiUpdateRecord Emoji = "📡" // updating DNS records
	EmojiClearRecord  Emoji = "🧹" // clearing DNS records

	EmojiPing         Emoji = "🔔" // pinging and health checks
	EmojiNotification Emoji = "📨" // notifications

	EmojiSignal      Emoji = "🚨" // catching signals
	EmojiAlreadyDone Emoji = "🤷" // DNS records were already up to date
	EmojiNow         Emoji = "🏃" // an event that is happening now or immediately
	EmojiAlarm       Emoji = "⏰" // an event that is scheduled to happen, but not immediately
	EmojiBye         Emoji = "👋" // bye!

	EmojiGood        Emoji = "😊" // good news
	EmojiUserError   Emoji = "😡" // configuration mistakes made by users
	EmojiUserWarning Emoji = "😦" // warnings about possible configuration mistakes
	EmojiError       Emoji = "😞" // errors that are not (directly) caused by user errors
	EmojiWarning     Emoji = "😐" // warnings about something unusual
	EmojiImpossible  Emoji = "🤯" // the impossible happened
	EmojiHint        Emoji = "💡" // Hints
)

// indentPrefix should be wider than an emoji to achieve visually pleasing results.
const indentPrefix = "   "
