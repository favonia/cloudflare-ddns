package file_test

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns-go/internal/file"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

func mockFS() afero.Fs {
	memfs := afero.NewMemMapFs()
	file.FS = afero.NewReadOnlyFs(memfs)
	return memfs
}

func resetFS() {
	file.FS = afero.NewOsFs()
}

//nolint:paralleltest // changing file.FS
func TestReadStringSuccessful(t *testing.T) {
	fs := mockFS()
	defer resetFS()

	path := "/etc/file.txt"
	written := " hello world   " // space is intentional
	expected := "hello world"

	err := afero.WriteFile(fs, path, []byte(written), 0644)
	require.NoError(t, err)

	content, ok := file.ReadString(pp.NoIndent, path)
	require.True(t, ok)
	require.Equal(t, expected, content)
}

//nolint:paralleltest // changing file.FS
func TestReadStringFailing(t *testing.T) {
	_ = mockFS()
	defer resetFS()

	path := "/wrong/path.txt"
	_, ok := file.ReadString(pp.NoIndent, path)
	require.False(t, ok)
}
