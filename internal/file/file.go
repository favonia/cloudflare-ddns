package file

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

const LinuxRoot string = "/"

var FS = os.DirFS(LinuxRoot) //nolint:gochecknoglobals

func ReadString(ppfmt pp.PP, path string) (string, bool) {
	// os.DirFS(...).Open() does not accept absolute paths
	if filepath.IsAbs(path) {
		newpath, err := filepath.Rel(LinuxRoot, path)
		if err != nil {
			ppfmt.Errorf(pp.EmojiImpossible, `%q is an absolute path but does not start with %q: %v`, path, LinuxRoot, err)
			return "", false
		}
		path = newpath
	}

	body, err := fs.ReadFile(FS, path)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to read %q: %v", path, err)
		return "", false
	}

	return string(bytes.TrimSpace(body)), true
}
