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
		"info/notice": {pp.Info, pp.Notice, true},
		"notice/info": {pp.Notice, pp.Info, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			fmt := pp.New(&buf, true, tc.set)

			require.Equal(t, tc.expected, fmt.IsShowing(tc.test))
		})
	}
}

func TestIndent(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	outer := pp.New(&buf, true, pp.DefaultVerbosity)

	outer.Noticef(pp.EmojiStar, "message1")
	middle := outer.Indent()
	middle.Noticef(pp.EmojiStar, "message2")
	inner := middle.Indent()
	outer.Noticef(pp.EmojiStar, "message3")
	outer.BlankLineIfVerbose()
	inner.Noticef(pp.EmojiStar, "message4")
	inner.BlankLineIfVerbose()
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
			fmt := pp.New(&buf, tc.emoji, tc.verbosity)

			fmt.Infof(pp.EmojiStar, "info")
			fmt.Noticef(pp.EmojiStar, "notice")

			require.Equal(t, tc.expected, buf.String())
		})
	}
}

func TestSupress(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	fmt := pp.New(&buf, true, pp.Info)

	fmt.Suppress(pp.ID(0))
	fmt.NoticeOncef(pp.ID(0), pp.EmojiAlarm, "hello %s", "world")
	fmt.InfoOncef(pp.ID(1), pp.EmojiHint, "hello %s", "galaxy")
	fmt.NoticeOncef(pp.ID(1), pp.EmojiBullet, "hello %s", "universe")
	fmt.NoticeOncef(pp.ID(2), pp.EmojiBye, "aloha")

	require.Equal(t, "💡 hello galaxy\n👋 aloha\n", buf.String())
}
