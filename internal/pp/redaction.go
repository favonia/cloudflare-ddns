package pp

type RedactionLevel int

const (
	RedactNone RedactionLevel = iota
	// This level reveals everything.

	RedactToken
	// This level conceals tokens or token-like information.

	RedactPrivate
	// This level conceals information that cannot be easily
	// owned by other people at the same time, including
	// token-like information, domain names, IP addresses,
	// zone IDs, and record IDs.

	RedactDefault RedactionLevel = RedactToken
)
