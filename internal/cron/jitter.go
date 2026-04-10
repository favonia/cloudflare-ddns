package cron

import (
	"math/rand"
	"time"
)

// JitterDuration returns a random duration in [0, interval/5) to spread
// API calls across clients that share the same nominal update interval.
// This reduces synchronized traffic spikes at Cloudflare's DNS API.
// Returns 0 for intervals too small to divide meaningfully (< 5ns).
func JitterDuration(interval time.Duration) time.Duration {
	if divisor := int64(interval) / 5; divisor > 0 {
		return time.Duration(rand.Int63n(divisor)) //nolint:gosec // non-cryptographic jitter
	}
	return 0
}
