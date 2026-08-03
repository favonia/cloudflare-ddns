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

func TestRequestAndDrain(t *testing.T) {
	t.Parallel()

	ppfmt := pp.NewSilent()
	id := pp.ID(0)

	// Mutation caught: draining an untouched ID must not manufacture a request.
	require.Equal(t, uint(0), ppfmt.DrainRequests(id))

	ppfmt.Request(id)
	requests := ppfmt.DrainRequests(id)
	require.Positive(t, requests)
	require.Equal(t, uint(1), requests)
	// Mutation caught: draining must clear the accumulated count.
	require.Equal(t, uint(0), ppfmt.DrainRequests(id))

	ppfmt.Request(id)
	ppfmt.Request(id)
	requests = ppfmt.DrainRequests(id)
	require.Positive(t, requests)
	require.Equal(t, uint(2), requests)

	// Mutation caught: a request made after a drain must remain observable.
	ppfmt.Request(id)
	requests = ppfmt.DrainRequests(id)
	require.Positive(t, requests)
	require.Equal(t, uint(1), requests)
}

func TestDrainRequestsDoesNotMarkOnceMessageSeen(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	ppfmt := pp.New(&output, false, pp.Verbose)
	id := pp.ID(0)

	require.Equal(t, uint(0), ppfmt.DrainRequests(id))
	ppfmt.NoticeOncef(id, pp.EmojiHint, "notice remains eligible")

	require.Equal(t, "notice remains eligible\n", output.String())
}

func TestRequestsAreIndependentFromOnceState(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	ppfmt := pp.New(&output, false, pp.Verbose)

	ppfmt.Request(pp.ID(0))
	ppfmt.NoticeOncef(pp.ID(0), pp.EmojiHint, "notice")
	requests := ppfmt.DrainRequests(pp.ID(0))
	require.Positive(t, requests)
	require.Equal(t, uint(1), requests)
	ppfmt.Request(pp.ID(0))
	ppfmt.InfoOncef(pp.ID(0), pp.EmojiHint, "suppressed info")
	requests = ppfmt.DrainRequests(pp.ID(0))
	require.Positive(t, requests)
	require.Equal(t, uint(1), requests)

	ppfmt.Request(pp.ID(1))
	ppfmt.InfoOncef(pp.ID(1), pp.EmojiHint, "info")
	requests = ppfmt.DrainRequests(pp.ID(1))
	require.Positive(t, requests)
	require.Equal(t, uint(1), requests)
	ppfmt.NoticeOncef(pp.ID(1), pp.EmojiHint, "suppressed notice")

	ppfmt.Suppress(pp.ID(2))
	ppfmt.Request(pp.ID(2))
	ppfmt.Request(pp.ID(2))
	requests = ppfmt.DrainRequests(pp.ID(2))
	require.Positive(t, requests)
	require.Equal(t, uint(2), requests)
	ppfmt.NoticeOncef(pp.ID(2), pp.EmojiHint, "suppressed notice")
	ppfmt.InfoOncef(pp.ID(2), pp.EmojiHint, "suppressed info")

	require.Equal(t, "notice\ninfo\n", output.String())
}

func TestDrainRequestsIsKeyedAndIndependentFromOnceState(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	ppfmt := pp.New(&output, false, pp.Verbose)
	idA := pp.ID(0)
	idB := pp.ID(1)

	ppfmt.Request(idA)
	ppfmt.Request(idA)
	ppfmt.Request(idB)
	ppfmt.Request(idB)
	ppfmt.Request(idB)

	require.Equal(t, uint(2), ppfmt.DrainRequests(idA))
	ppfmt.NoticeOncef(idA, pp.EmojiHint, "notice after drain")
	require.Equal(t, "notice after drain\n", output.String())
	require.Equal(t, uint(3), ppfmt.DrainRequests(idB))

	ppfmt.Request(idA)
	require.Equal(t, uint(1), ppfmt.DrainRequests(idA))
	ppfmt.NoticeOncef(idA, pp.EmojiHint, "suppressed notice")
	require.Equal(t, "notice after drain\n", output.String())
}

func TestIndentedPrintersShareRequests(t *testing.T) {
	t.Parallel()

	outer := pp.NewSilent()
	inner := outer.Indent()
	id := pp.ID(0)

	outer.Request(id)
	inner.Request(id)
	requests := inner.DrainRequests(id)
	require.Positive(t, requests)
	require.Equal(t, uint(2), requests)
	require.Equal(t, uint(0), outer.DrainRequests(id))

	inner.Request(id)
	requests = outer.DrainRequests(id)
	require.Positive(t, requests)
	require.Equal(t, uint(1), requests)
}

func TestRequestsIgnoreVerbosity(t *testing.T) {
	t.Parallel()

	var quietOutput strings.Builder
	for name, ppfmt := range map[string]pp.PP{
		"quiet":  pp.New(&quietOutput, false, pp.Quiet),
		"silent": pp.NewSilent(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ppfmt.Request(pp.ID(0))
			ppfmt.Request(pp.ID(0))
			requests := ppfmt.DrainRequests(pp.ID(0))
			require.Positive(t, requests)
			require.Equal(t, uint(2), requests)
			require.Equal(t, uint(0), ppfmt.DrainRequests(pp.ID(0)))
		})
	}
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
