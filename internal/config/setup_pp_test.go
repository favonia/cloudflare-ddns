package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest // environment vars are global
func TestSetupPP(t *testing.T) {
	unset(t, "EMOJI", "QUIET")

	var buf strings.Builder

	ppfmt, ok := config.SetupPP(&buf)
	require.True(t, ok)
	require.NotNil(t, ppfmt)

	ppfmt.Errorf(pp.EmojiStar, "message")
	require.Equal(t, `🌟 message
`, buf.String())
}

//nolint:paralleltest // environment vars are global
func TestSetupPPInvalidEmoji(t *testing.T) {
	set(t, "EMOJI", true, "invalid")
	set(t, "QUIET", true, "invalid")

	var buf strings.Builder
	ppfmt, ok := config.SetupPP(&buf)
	require.False(t, ok)
	require.Nil(t, ppfmt)
	require.Equal(t,
		`😡 EMOJI ("invalid") is not a boolean: strconv.ParseBool: parsing "invalid": invalid syntax
`,
		buf.String())
}
