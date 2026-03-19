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
	outer := pp.New(&buf, true, pp.Verbose)

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

func TestOnceAndSuppress(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	fmt := pp.New(&buf, true, pp.Info)

	fmt.NoticeOncef(pp.ID(0), pp.EmojiAlarm, "hello %s", "world")
	fmt.NoticeOncef(pp.ID(0), pp.EmojiAlarm, "hello %s", "do not print")

	fmt.Suppress(pp.ID(1))
	fmt.InfoOncef(pp.ID(1), pp.EmojiHint, "hello %s", "do not print")

	fmt.InfoOncef(pp.ID(2), pp.EmojiHint, "hello %s", "galaxy")
	fmt.NoticeOncef(pp.ID(2), pp.EmojiBullet, "hello %s", "universe")
	fmt.NoticeOncef(pp.ID(3), pp.EmojiBye, "aloha")

	require.Equal(t, "⏰ hello world\n💡 hello galaxy\n👋 aloha\n", buf.String())
}

func TestNewDefault(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	fmt := pp.NewDefault(&buf)

	fmt.Infof(pp.EmojiStar, "hello")
	fmt.Noticef(pp.EmojiStar, "world")

	require.Equal(t, "🌟 hello\n🌟 world\n", buf.String())
}

func TestNewSilent(t *testing.T) {
	t.Parallel()

	fmt := pp.NewSilent()

	require.False(t, fmt.IsShowing(pp.Notice))
	require.False(t, fmt.IsShowing(pp.Info))
	require.False(t, fmt.Indent().IsShowing(pp.Notice))
	require.NotPanics(t, func() { fmt.Infof(pp.EmojiStar, "hello") })
	require.NotPanics(t, func() { fmt.Noticef(pp.EmojiStar, "world") })
	require.NotPanics(t, fmt.BlankLineIfVerbose)
	require.NotPanics(t, func() { fmt.Suppress(pp.ID(0)) })
	require.NotPanics(t, func() { fmt.InfoOncef(pp.ID(1), pp.EmojiHint, "once") })
	require.NotPanics(t, func() { fmt.NoticeOncef(pp.ID(2), pp.EmojiAlarm, "once") })
}
