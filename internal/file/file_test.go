package file_test

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func useMemFS(memfs fstest.MapFS) {
	file.FS = memfs
}

func useDirFS() {
	file.FS = os.DirFS("/")
}

//nolint:paralleltest // changing global var file.FS
func TestReadStringOkay(t *testing.T) {
	path := "test/file.txt"
	written := " hello world   " // space is intentionally added to test trimming
	expected := strings.TrimSpace(written)

	useMemFS(fstest.MapFS{
		path: &fstest.MapFile{
			Data:    []byte(written),
			Mode:    0o644,
			ModTime: time.Unix(1234, 5678),
			Sys:     nil,
		},
	})
	defer useDirFS()

	ppmock := pp.NewMock()
	content, ok := file.ReadString(ppmock, path)
	require.True(t, ok)
	require.Equal(t, expected, content)
	require.Empty(t, ppmock.Records)
}

//nolint:paralleltest // changing global var file.FS
func TestReadStringWrongPath(t *testing.T) {
	useMemFS(fstest.MapFS{})
	defer useDirFS()

	path := "/wrong/path.txt"
	ppmock := pp.NewMock()
	content, ok := file.ReadString(ppmock, path)
	require.False(t, ok)
	require.Empty(t, content)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to read "/wrong/path.txt": open /wrong/path.txt: file does not exist`), //nolint:lll
		},
		ppmock.Records)
}

//nolint:paralleltest // changing global var file.FS
func TestReadStringNoAccess(t *testing.T) {
	useMemFS(fstest.MapFS{
		"dir/file.txt": &fstest.MapFile{
			Data:    []byte("hello"),
			Mode:    0,
			ModTime: time.Unix(1234, 5678),
			Sys:     nil,
		},
	})
	defer useDirFS()

	ppmock := pp.NewMock()
	content, ok := file.ReadString(ppmock, "dir")
	require.False(t, ok)
	require.Empty(t, content)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to read "dir": read dir: invalid argument`),
		},
		ppmock.Records)
}
