package testenv_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/testenv"
)

//nolint:paralleltest // environment variables are global
func TestClearAllClearsAndRestoresEnvironment(t *testing.T) {
	const presentKey = "TESTENV_PRESENT_KEY"
	const absentKey = "TESTENV_ABSENT_KEY"

	t.Setenv(presentKey, "before")
	t.Setenv(absentKey, "")
	require.NoError(t, os.Unsetenv(absentKey))

	t.Run("clear", func(t *testing.T) {
		require.Equal(t, "before", os.Getenv(presentKey))
		require.Empty(t, os.Getenv(absentKey))

		testenv.ClearAll(t)

		require.Empty(t, os.Getenv(presentKey))
		require.Empty(t, os.Getenv(absentKey))

		t.Setenv(presentKey, "during")
		require.Equal(t, "during", os.Getenv(presentKey))
	})

	require.Equal(t, "before", os.Getenv(presentKey))
	require.Empty(t, os.Getenv(absentKey))
}
