package config_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns-go/internal/config"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

const keyPrefix = "TEST-11d39f6a9a97afafd87cceb-"

func set(key string, val string) {
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
	key := keyPrefix + "VAR"
	for name, tc := range map[string]struct {
		set      bool
		val      string
		expected string
	}{
		"nil":    {false, "", ""},
		"empty":  {true, "", ""},
		"simple": {true, "VAL", "VAL"},
		"space1": {true, "    VAL     ", "VAL"},
		"space2": {true, "     VAL    VAL2 ", "VAL    VAL2"},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}
			require.Equal(t, tc.expected, config.Getenv(key))
		})
	}
}

//nolint: paralleltest // environment vars are global
func TestReadQuiet(t *testing.T) {
	key := keyPrefix + "TEST_QUIET"
	for name, tc := range map[string]struct {
		set      bool
		val      string
		oldField quiet.Quiet
		newField quiet.Quiet
		ok       bool
	}{
		"nil1":     {false, "", quiet.VERBOSE, quiet.VERBOSE, true},
		"nil2":     {false, "", quiet.QUIET, quiet.QUIET, true},
		"empty1":   {true, "", quiet.VERBOSE, quiet.VERBOSE, true},
		"empty2":   {true, "", quiet.QUIET, quiet.QUIET, true},
		"true1":    {true, "true", quiet.VERBOSE, quiet.QUIET, true},
		"true2":    {true, "true", quiet.QUIET, quiet.QUIET, true},
		"false1":   {true, "false", quiet.VERBOSE, quiet.VERBOSE, true},
		"false2":   {true, "false", quiet.QUIET, quiet.VERBOSE, true},
		"illform1": {true, "weird", quiet.QUIET, quiet.QUIET, false},
		"illform2": {true, "weird", quiet.VERBOSE, quiet.VERBOSE, false},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			field := tc.oldField
			ok := config.ReadQuiet(pp.Indent(1), key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}
