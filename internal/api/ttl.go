package api

import "strconv"

// A TTL represents a time-to-live value of a DNS record.
type TTL int

// TTLAuto represents the "auto" value for Cloudflare servers.
const TTLAuto TTL = 1

// Int converts a TTL into its raw integer value.
func (t TTL) Int() int {
	return int(t)
}

// String converts a TTL into the string representation of its raw integer value.
func (t TTL) String() string {
	return strconv.Itoa(t.Int())
}

// Describe converts a TTL into a human-readable, user-friendly description
// that is suitable for printing.
func (t TTL) Describe() string {
	if t == TTLAuto {
		return "1 (auto)"
	}
	return strconv.Itoa(t.Int())
}
