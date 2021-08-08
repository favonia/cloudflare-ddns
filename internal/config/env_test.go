package config_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/quiet"
)

const keyPrefix = "TEST-11D39F6A9A97AFAFD87CCEB-"

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
	key := keyPrefix + "QUIET"
	for name, tc := range map[string]struct {
		set      bool
		val      string
		oldField quiet.Quiet
		newField quiet.Quiet
		ok       bool
	}{
		"nil1":     {false, "", quiet.VERBOSE, quiet.VERBOSE, true},
		"nil2":     {false, "", quiet.QUIET, quiet.QUIET, true},
		"empty1":   {true, "  ", quiet.VERBOSE, quiet.VERBOSE, true},
		"empty2":   {true, " ", quiet.QUIET, quiet.QUIET, true},
		"true1":    {true, "true   ", quiet.VERBOSE, quiet.QUIET, true},
		"true2":    {true, " true", quiet.QUIET, quiet.QUIET, true},
		"false1":   {true, "    false ", quiet.VERBOSE, quiet.VERBOSE, true},
		"false2":   {true, " false    ", quiet.QUIET, quiet.VERBOSE, true},
		"illform1": {true, "weird", quiet.VERBOSE, quiet.VERBOSE, false},
		"illform2": {true, "weird", quiet.QUIET, quiet.QUIET, false},
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

//nolint: paralleltest // environment vars are global
func TestReadString(t *testing.T) {
	key := keyPrefix + "STRING"
	for name, tc := range map[string]struct {
		set      bool
		val      string
		quiet    quiet.Quiet
		oldField string
		newField string
		ok       bool
	}{
		"nil1":    {false, "", quiet.VERBOSE, "original", "original", true},
		"nil2":    {false, "", quiet.QUIET, "original", "original", true},
		"empty1":  {true, "  ", quiet.VERBOSE, "original", "original", true},
		"empty2":  {true, " ", quiet.QUIET, "original", "original", true},
		"random1": {true, " ran dom ", quiet.VERBOSE, "original", "ran dom", true},
		"random2": {true, "  random", quiet.QUIET, "original", "random", true},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			field := tc.oldField
			ok := config.ReadString(tc.quiet, pp.Indent(1), key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint: paralleltest // environment vars are global
func TestReadBool(t *testing.T) {
	key := keyPrefix + "BOOL"
	for name, tc := range map[string]struct {
		set      bool
		val      string
		quiet    quiet.Quiet
		oldField bool
		newField bool
		ok       bool
	}{
		"nil1":     {false, "", quiet.VERBOSE, true, true, true},
		"nil2":     {false, "", quiet.QUIET, false, false, true},
		"empty1":   {true, " ", quiet.VERBOSE, true, true, true},
		"empty2":   {true, " \t ", quiet.QUIET, false, false, true},
		"true1":    {true, "true ", quiet.VERBOSE, true, true, true},
		"true2":    {true, " \t true", quiet.QUIET, false, true, true},
		"false1":   {true, "false ", quiet.VERBOSE, true, false, true},
		"false2":   {true, " false", quiet.QUIET, false, false, true},
		"illform1": {true, "weird\t  ", quiet.VERBOSE, false, false, false},
		"illform2": {true, " weird", quiet.QUIET, true, true, false},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			field := tc.oldField
			ok := config.ReadBool(tc.quiet, pp.Indent(1), key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint: paralleltest // environment vars are global
func TestReadNonnegInt(t *testing.T) {
	key := keyPrefix + "INT"
	for name, tc := range map[string]struct {
		set      bool
		val      string
		quiet    quiet.Quiet
		oldField int
		newField int
		ok       bool
	}{
		"nil-quiet":     {false, "", quiet.QUIET, 100, 100, true},
		"nil-verbose":   {false, "", quiet.VERBOSE, 100, 100, true},
		"empty-quiet":   {true, "", quiet.QUIET, 100, 100, true},
		"empty-verbose": {true, "", quiet.VERBOSE, 100, 100, true},
		"zero":          {true, "0   ", quiet.VERBOSE, 100, 0, true},
		"-1-quiet":      {true, "   -1", quiet.QUIET, 100, 100, false},
		"-1-verbose":    {true, "   -1", quiet.VERBOSE, 100, 100, false},
		"1":             {true, "   1   ", quiet.VERBOSE, 100, 1, true},
		"1.0":           {true, "   1.0   ", quiet.VERBOSE, 100, 100, false},
		"words-quiet":   {true, "   words   ", quiet.QUIET, 100, 100, false},
		"words-verbose": {true, "   words   ", quiet.VERBOSE, 100, 100, false},
		"9999999":       {true, "   9999999   ", quiet.VERBOSE, 100, 9999999, true},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			field := tc.oldField
			ok := config.ReadNonnegInt(tc.quiet, pp.NoIndent, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint: paralleltest // environment vars are global
func TestReadDomains(t *testing.T) {
	key := keyPrefix + "DOMAINS"
	type ds = []api.FQDN
	for name, tc := range map[string]struct {
		set      bool
		val      string
		quiet    quiet.Quiet
		oldField ds
		newField ds
		ok       bool
	}{
		"nil-quiet":   {false, "", quiet.QUIET, ds{"test.org"}, ds{}, true},
		"nil-verbose": {false, "", quiet.VERBOSE, ds{"test.org"}, ds{}, true},
		"empty":       {true, "", quiet.VERBOSE, ds{"test.org"}, ds{}, true},
		"test1":       {true, "書.org ,  Bücher.org  ", quiet.VERBOSE, ds{"random.org"}, ds{"xn--rov.org", "xn--bcher-kva.org"}, true},
		"test2":       {true, "  \txn--rov.org    ,   xn--Bcher-kva.org  ", quiet.VERBOSE, ds{"random.org"}, ds{"xn--rov.org", "xn--bcher-kva.org"}, true},
		"illformed":   {true, "xn--:D.org", quiet.VERBOSE, ds{"random.org"}, ds{"xn--:d.org"}, true},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.set {
				set(key, tc.val)
				defer unset(key)
			}

			field := tc.oldField
			ok := config.ReadDomains(quiet.QUIET, pp.NoIndent, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}
