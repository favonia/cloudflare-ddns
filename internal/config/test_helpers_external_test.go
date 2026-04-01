package config_test

import (
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
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

func unset(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		set(t, k, false, "")
	}
}

func urlMustParse(t *testing.T, u string) *url.URL {
	t.Helper()
	url, err := url.Parse(u)
	require.NoError(t, err)
	return url
}
