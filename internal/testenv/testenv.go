package testenv

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// ClearAll clears the current environment and restores the original values
// during cleanup.
func ClearAll(tb testing.TB) {
	tb.Helper()

	originalEnv := slices.Clone(os.Environ())
	for _, entry := range originalEnv {
		key, _, _ := strings.Cut(entry, "=")
		tb.Setenv(key, "")
	}
}
