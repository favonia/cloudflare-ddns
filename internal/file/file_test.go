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
	content, ok := file.ReadString(mockPP, path)
	require.True(t, ok)
	require.Equal(t, expected, content)
}

//nolint:paralleltest // changing global var file.FS
func TestReadStringWrongPath(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	useMemFS(t, fstest.MapFS{})

	path := "wrong/path.txt"
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiUserError, "Failed to read %q: %v", path, gomock.Any())
	content, ok := file.ReadString(mockPP, path)
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
	mockPP.EXPECT().Noticef(pp.EmojiUserError, "Failed to read %q: %v", "dir", gomock.Any())
	content, ok := file.ReadString(mockPP, "dir")
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
