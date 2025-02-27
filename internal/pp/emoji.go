package pp

// Emoji is the type of emoji strings.
type Emoji string

// Various constants defining emojis used in the updater.
const (
	EmojiStar   Emoji = "🌟" // stars attached to the updater name
	EmojiBullet Emoji = "🔸" // generic bullet points

	EmojiEnvVars      Emoji = "📖" // reading configuration
	EmojiConfig       Emoji = "🔧" // showing configuration
	EmojiInternet     Emoji = "🌐" // network address detection
	EmojiMute         Emoji = "🔇" // quiet mode
	EmojiDisabled     Emoji = "🚫" // feature is disabled
	EmojiExperimental Emoji = "🧪" // experimental features
	EmojiSwitch       Emoji = "🔀" // the happy eyeballs algorithm chose the alternative
	EmojiTruncate     Emoji = "🪚" // Truncate IPv6 host IDs

	EmojiCreation Emoji = "🐣" // adding new DNS records
	EmojiDeletion Emoji = "💀" // deleting DNS records
	EmojiUpdate   Emoji = "📡" // updating DNS records
	EmojiClear    Emoji = "🧹" // clearing DNS records when exiting

	EmojiPing   Emoji = "🔔" // pinging and health checks
	EmojiNotify Emoji = "📣" // notifications

	EmojiTimeout     Emoji = "⌛" // Timeout or abortion
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
