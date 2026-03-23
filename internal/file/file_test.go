package file_test

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func useMemFS(t *testing.T, memfs fstest.MapFS) {
	t.Helper()
	file.FS = memfs
	t.Cleanup(func() { file.FS = os.DirFS("/") })
}

//nolint:paralleltest // changing global var file.FS
func TestReadString(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	path := "test/file.txt"
	written := " hello world   " // space is intentionally added to test trimming
	expected := strings.TrimSpace(written)

	useMemFS(t, fstest.MapFS{
		path: &fstest.MapFile{
			Data:    []byte(written),
			Mode:    0o644,
			ModTime: time.Unix(1234, 5678),
			Sys:     nil,
		},
	})

	mockPP := mocks.NewMockPP(mockCtrl)
	content, ok := file.ReadString(mockPP, "/"+path)
	require.True(t, ok)
	require.Equal(t, expected, content)
}

//nolint:paralleltest // changing global var file.FS
func TestReadStringWrongPath(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	useMemFS(t, fstest.MapFS{})

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiUserError, "Failed to read %q: %v", "/wrong/path.txt", gomock.Any())
	content, ok := file.ReadString(mockPP, "/wrong/path.txt")
	require.False(t, ok)
	require.Empty(t, content)
}

//nolint:paralleltest // changing global var file.FS
func TestReadStringNoAccess(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	useMemFS(t, fstest.MapFS{
		"dir/file.txt": &fstest.MapFile{
			Data:    []byte("hello"),
			Mode:    0,
			ModTime: time.Unix(1234, 5678),
			Sys:     nil,
		},
	})

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiUserError, "Failed to read %q: %v", "/dir", gomock.Any())
	content, ok := file.ReadString(mockPP, "/dir")
	require.False(t, ok)
	require.Empty(t, content)
}

//nolint:paralleltest // reading global var file.FS
func TestReadStringAbsolutePath(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	path := "test/file.txt"
	written := " hello world   " // space is intentionally added to test trimming
	expected := strings.TrimSpace(written)

	useMemFS(t, fstest.MapFS{
		path: &fstest.MapFile{
			Data:    []byte(written),
			Mode:    0o644,
			ModTime: time.Unix(1234, 5678),
			Sys:     nil,
		},
	})

	mockPP := mocks.NewMockPP(mockCtrl)
	content, ok := file.ReadString(mockPP, "/"+path)
	require.True(t, ok)
	require.Equal(t, expected, content)
}

func TestReadStringRelativePath(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiUserError,
		"The path %q is not absolute; to use an absolute path, prefix it with /", "relative/path.txt")
	content, ok := file.ReadString(mockPP, "relative/path.txt")
	require.False(t, ok)
	require.Empty(t, content)
}

func collectLines(lines func(func(int, string) bool)) []struct {
	num  int
	text string
} {
	var result []struct {
		num  int
		text string
	}
	for num, text := range lines {
		result = append(result, struct {
			num  int
			text string
		}{num, text})
	}
	return result
}

//nolint:paralleltest // changing global var file.FS
func TestReadLines(t *testing.T) {
	for name, tc := range map[string]struct {
		content  string
		expected []struct {
			num  int
			text string
		}
	}{
		"normal": {
			"1.1.1.1\n2.2.2.2\n",
			[]struct {
				num  int
				text string
			}{
				{1, "1.1.1.1"},
				{2, "2.2.2.2"},
			},
		},
		"comments": {
			"# this is a comment\n1.1.1.1\n# another comment\n",
			[]struct {
				num  int
				text string
			}{
				{2, "1.1.1.1"},
			},
		},
		"inline-comment": {
			"1.1.1.1 # home gateway\n",
			[]struct {
				num  int
				text string
			}{
				{1, "1.1.1.1"},
			},
		},
		"blank-lines": {
			"\n\n1.1.1.1\n\n2.2.2.2\n\n",
			[]struct {
				num  int
				text string
			}{
				{3, "1.1.1.1"},
				{5, "2.2.2.2"},
			},
		},
		"whitespace": {
			"  1.1.1.1  \n\t2.2.2.2\t\n",
			[]struct {
				num  int
				text string
			}{
				{1, "1.1.1.1"},
				{2, "2.2.2.2"},
			},
		},
		"comment-only": {
			"# comment\n# another\n",
			nil,
		},
		"empty": {
			"",
			nil,
		},
		"no-trailing-newline": {
			"1.1.1.1",
			[]struct {
				num  int
				text string
			}{
				{1, "1.1.1.1"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			useMemFS(t, fstest.MapFS{
				"ips.txt": &fstest.MapFile{
					Data:    []byte(tc.content),
					Mode:    0o644,
					ModTime: time.Unix(1234, 5678),
					Sys:     nil,
				},
			})

			mockPP := mocks.NewMockPP(mockCtrl)
			lines, ok := file.ReadLines(mockPP, "/ips.txt")
			require.True(t, ok)
			require.Equal(t, tc.expected, collectLines(lines))
		})
	}
}

//nolint:paralleltest // changing global var file.FS
func TestReadLinesWrongPath(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	useMemFS(t, fstest.MapFS{})

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiUserError, "Failed to read %q: %v", "/missing.txt", gomock.Any())
	lines, ok := file.ReadLines(mockPP, "/missing.txt")
	require.False(t, ok)
	require.Nil(t, lines)
}

func TestReadLinesRelativePath(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiUserError,
		"The path %q is not absolute; to use an absolute path, prefix it with /", "relative.txt")
	lines, ok := file.ReadLines(mockPP, "relative.txt")
	require.False(t, ok)
	require.Nil(t, lines)
}
