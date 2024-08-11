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
		set      pp.Verbosity
		test     pp.Verbosity
		expected bool
	}{
		"info-notice": {pp.Info, pp.Notice, true},
		"erorr-info":  {pp.Error, pp.Info, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			fmt := pp.New(&buf).SetVerbosity(tc.set)

			require.Equal(t, tc.expected, fmt.IsEnabledFor(tc.test))
		})
	}
}

func TestIndent(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	outer := pp.New(&buf)

	outer.Errorf(pp.EmojiStar, "message1")
	middle := outer.Indent()
	middle.Errorf(pp.EmojiStar, "message2")
	inner := middle.Indent()
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
		emoji     bool
		verbosity pp.Verbosity
		expected  string
	}{
		"info":              {true, pp.Info, "🌟 info\n🌟 notice\n🌟 warning\n🌟 error\n"},
		"notice":            {true, pp.Notice, "🌟 notice\n🌟 warning\n🌟 error\n"},
		"warning":           {true, pp.Warning, "🌟 warning\n🌟 error\n"},
		"errorfmt":          {true, pp.Error, "🌟 error\n"},
		"info/no-emoji":     {false, pp.Info, "info\nnotice\nwarning\nerror\n"},
		"notice/no-emoji":   {false, pp.Notice, "notice\nwarning\nerror\n"},
		"warning/no-emoji":  {false, pp.Warning, "warning\nerror\n"},
		"errorfmt/no-emoji": {false, pp.Error, "error\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			fmt := pp.New(&buf).SetEmoji(tc.emoji).SetVerbosity(tc.verbosity)

			fmt.Infof(pp.EmojiStar, "info")
			fmt.Noticef(pp.EmojiStar, "notice")
			fmt.Warningf(pp.EmojiStar, "warning")
			fmt.Errorf(pp.EmojiStar, "error")

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
