package pp_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestIsEnabledFor(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		set      pp.Level
		test     pp.Level
		expected bool
	}{
		"info-notice": {pp.Info, pp.Notice, true},
		"erorr-info":  {pp.Error, pp.Info, false},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			fmt := pp.New(&buf).SetLevel(tc.set)

			require.Equal(t, tc.expected, fmt.IsEnabledFor(tc.test))
		})
	}
}

func TestIncIndent(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	outer := pp.New(&buf)

	outer.Errorf(pp.EmojiStar, "message1")
	middle := outer.IncIndent()
	middle.Errorf(pp.EmojiStar, "message2")
	inner := middle.IncIndent()
	outer.Errorf(pp.EmojiStar, "message3")
	inner.Errorf(pp.EmojiStar, "message4")
	middle.Errorf(pp.EmojiStar, "message5")

	require.Equal(t,
		`🌟 message1
   🌟 message2
🌟 message3
      🌟 message4
   🌟 message5
`,
		buf.String())
}

func TestPrint(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		level    pp.Level
		expected string
	}{
		"info":     {pp.Info, "🌟 info\n🌟 notice\n🌟 warning\n🌟 error\n"},
		"notice":   {pp.Notice, "🌟 notice\n🌟 warning\n🌟 error\n"},
		"warning":  {pp.Warning, "🌟 warning\n🌟 error\n"},
		"errorfmt": {pp.Error, "🌟 error\n"},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			fmt := pp.New(&buf).SetLevel(tc.level)

			fmt.Infof(pp.EmojiStar, "info")
			fmt.Noticef(pp.EmojiStar, "notice")
			fmt.Warningf(pp.EmojiStar, "warning")
			fmt.Errorf(pp.EmojiStar, "error")

			require.Equal(t, tc.expected, buf.String())
		})
	}
}
