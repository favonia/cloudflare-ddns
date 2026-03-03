package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
)

func TestDefaultConfigNotNil(t *testing.T) {
	t.Parallel()

	require.NotNil(t, config.DefaultRaw())
}

func TestDefaultConfigMonitorNotNil(t *testing.T) {
	t.Parallel()

	require.NotNil(t, config.DefaultRaw().Monitor)
}

func TestDefaultConfigoNotifierNotNil(t *testing.T) {
	t.Parallel()

	require.NotNil(t, config.DefaultRaw().Notifier)
}
