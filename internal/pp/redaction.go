package pp

type PrivateDataType uint32

const (
	Tokens PrivateDataType = 1 << iota
	// Tokens (special strings that grant access or prove identities).

	IPs
	// IP addresses.

	Domains
	// Domain names.

	LinuxIDs
	// User IDs and group IDs.

	DNSResourceIDs
	// IDs of records, zones, etc. that are not tokens.
)

type RedactMask uint32

const (
	RedactNone RedactMask = 0
	// This level reveals everything.

	RedactTokens RedactMask = RedactMask(Tokens)
	// This level removes token-like information.

	RedactMaximum RedactMask = ^RedactMask(0)
	// This level removes information that cannot be easily
	// owned by other people at the same time, including
	// token-like information, domain names, IP addresses,
	// zone IDs, and record IDs.

	DefaultRedactMask RedactMask = RedactTokens
)
