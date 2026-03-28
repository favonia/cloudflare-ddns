package testenv

import (
	"os"
	"strings"
	"testing"
)

// ClearAll snapshots the current environment, clears it for the current test,
// and restores the original values during cleanup.
func ClearAll(t testing.TB) {
	t.Helper()

	originalEnv := append([]string(nil), os.Environ()...)
	os.Clearenv()

	t.Cleanup(func() {
		os.Clearenv()
		for _, entry := range originalEnv {
			key, value, ok := strings.Cut(entry, "=")
			if !ok {
				continue
			}
			_ = os.Setenv(key, value)
		}
	})
}
