package pp_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestIsShowing(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		set      pp.Verbosity
		test     pp.Verbosity
		expected bool
	}{
		"info-notice": {pp.Info, pp.Notice, true},
		"notice-info": {pp.Notice, pp.Info, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			fmt := pp.New(&buf).SetVerbosity(tc.set)

			require.Equal(t, tc.expected, fmt.IsShowing(tc.test))
		})
	}
}

func TestIndent(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	outer := pp.New(&buf)

	outer.Noticef(pp.EmojiStar, "message1")
	middle := outer.Indent()
	middle.Noticef(pp.EmojiStar, "message2")
	inner := middle.Indent()
	outer.Noticef(pp.EmojiStar, "message3")
	inner.Noticef(pp.EmojiStar, "message4")
	middle.Noticef(pp.EmojiStar, "message5")

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
		emoji     bool
		verbosity pp.Verbosity
		expected  string
	}{
		"info":            {true, pp.Info, "🌟 info\n🌟 notice\n"},
		"notice":          {true, pp.Notice, "🌟 notice\n"},
		"info/no-emoji":   {false, pp.Info, "info\nnotice\n"},
		"notice/no-emoji": {false, pp.Notice, "notice\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			fmt := pp.New(&buf).SetEmoji(tc.emoji).SetVerbosity(tc.verbosity)

			fmt.Infof(pp.EmojiStar, "info")
			fmt.Noticef(pp.EmojiStar, "notice")

			require.Equal(t, tc.expected, buf.String())
		})
	}
}

func TestSupressHint(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	fmt := pp.New(&buf).SetEmoji(true).SetVerbosity(pp.Info)

	fmt.SuppressHint(pp.Hint(0))
	fmt.Hintf(pp.Hint(0), "hello %s", "world")
	fmt.Hintf(pp.Hint(1), "hello %s", "galaxy")
	fmt.Hintf(pp.Hint(1), "hello %s", "universe")

	require.Equal(t, "💡 hello galaxy\n", buf.String())
}
