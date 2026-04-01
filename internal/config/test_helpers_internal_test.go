package config

import (
	"os"
	"testing"
)

func set(t *testing.T, key string, set bool, val string) {
	t.Helper()

	if set {
		t.Setenv(key, val)
	} else {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func store(t *testing.T, key string, val string) { t.Helper(); set(t, key, true, val) }
