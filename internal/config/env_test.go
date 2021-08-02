package config_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns-go/internal/config"
)

func set(key, val string) {
	if os.Getenv(key) != "" {
		panic(fmt.Sprintf("%s was already set", key))
	}

	os.Setenv(key, val)
}

func unset(key string) {
	os.Unsetenv(key)
}

//nolint: paralleltest // environment vars are global
func TestGetenv(t *testing.T) {
	for name, tc := range map[string]struct {
		key      string
		val      string
		expected string
	}{
		"simple": {"TEST_VAR", "TEST_VAL", "TEST_VAL"},
		"space":  {"TEST_VAR", "     TEST_VAL     ", "TEST_VAL"},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			set(tc.key, tc.val)
			defer unset(tc.key)
			require.Equal(t, tc.expected, config.Getenv(tc.key))
		})
	}
}
